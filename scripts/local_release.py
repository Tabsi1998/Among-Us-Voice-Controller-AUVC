#!/usr/bin/env python3
"""Build a pre-release on this computer and, when asked, publish it.

Pre-releases, versions with -alpha, -beta or -rc such as v1.2.3-beta, are built
here: every check of local_check.py runs against a fresh copy of the commit,
the release dry run included, and only then are the installer, the portable
zip and SHA256SUMS uploaded. Releases without a suffix, such as v1.2.3, are not
built here: pushing their tag starts release.yml, which builds them on a clean
runner.

Usage:
    python scripts/local_release.py v1.2.3-beta             build and check, publish nothing
    python scripts/local_release.py v1.2.3-beta --publish   also tag, push the tag and upload

The build and the files stay in .local-testing/, which Git ignores.
"""

from __future__ import annotations

import argparse
import contextlib
import json
import os
import re
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

import local_check
import release_notes

ROOT = local_check.ROOT
BUILD = local_check.STATE / "release-build"

PRERELEASE = re.compile(r"^v\d+\.\d+\.\d+-(alpha|beta|rc)(\.\d+)?$")
RELEASE = re.compile(r"^v\d+\.\d+\.\d+$")
ASSETS = ("AmongUsVoiceCapture-Setup-win-x64.exe", "AmongUsVoiceCapture-win-x64.zip", "SHA256SUMS")
INNO_SETUP_INSTALL = "winget install JRSoftware.InnoSetup"


class Refused(Exception):
    """The release cannot go ahead. The message says why and what to do."""


def check_version(tag: str) -> None:
    if PRERELEASE.match(tag):
        return
    if RELEASE.match(tag):
        raise Refused(
            f"{tag} is a release, not a pre-release. Releases are built on GitHub: "
            f"tag the merge commit on main and push the tag (git push origin {tag})")
    raise Refused(
        f"{tag} is not a version this script publishes. Use v<major>.<minor>.<patch>-beta, "
        "-alpha or -rc, optionally numbered, for example v1.2.3-beta or v1.2.3-rc.1")


@dataclass
class State:
    """What the repository and GitHub say before anything is changed."""
    clean: bool
    head: str
    head_on_main: bool
    notes_found: bool
    tag_commit: str | None
    release_exists: bool
    gh_logged_in: bool
    inno_setup: bool


def refusals(tag: str, state: State, publish: bool) -> list[str]:
    reasons = []
    if not state.clean:
        reasons.append("the working folder has uncommitted changes. A release is built from a commit, "
                       "so commit or stash them first")
    if not state.head_on_main:
        reasons.append("this commit is not on origin/main. Merge the pull request with the "
                       "CHANGELOG section, then switch to main and pull")
    if not state.notes_found:
        reasons.append(f"CHANGELOG.md has no section for {tag}. Rename ## Unreleased to the version "
                       "and merge that first")
    if state.release_exists:
        reasons.append(f"a GitHub release {tag} already exists; releases are never replaced")
    if state.tag_commit and state.tag_commit != state.head:
        reasons.append(f"the tag {tag} already exists on another commit; releases are never moved")
    if publish and not state.gh_logged_in:
        reasons.append("the GitHub CLI is not logged in. Run gh auth login")
    if publish and not state.inno_setup:
        reasons.append(f"Inno Setup 6 is needed to build the installer. Install it with: {INNO_SETUP_INSTALL}")
    return reasons


def missing_assets(folder: Path) -> list[str]:
    return [name for name in ASSETS if not (folder / name).is_file()]


def unlisted_in_checksums(sums: str) -> list[str]:
    listed = {line.split(maxsplit=1)[1].strip().lstrip("*") for line in sums.splitlines() if len(line.split()) == 2}
    return [name for name in ASSETS if name != "SHA256SUMS" and name not in listed]


def failed_checks(report: dict, publish: bool) -> list[str]:
    """Checks that stop a release. Publishing also needs the installer to have been built."""
    problems = [f"{result['group']}: {result['describe']}"
                for result in report.get("results", []) if result["status"] == local_check.FAIL]
    if publish:
        installer = [result for result in report.get("results", []) if result["name"] == "installer"]
        if not installer or installer[0]["status"] != local_check.PASS:
            problems.append("release: the installer was not built")
    return problems


def run(command: list[str], cwd: Path = ROOT, env: dict[str, str] | None = None,
        quiet: bool = False) -> subprocess.CompletedProcess:
    if not quiet:
        print("> " + subprocess.list2cmdline(command), flush=True)
    return subprocess.run(command, cwd=cwd, env=env, text=True, encoding="utf-8", errors="replace",
                          capture_output=quiet)


def output(command: list[str]) -> str:
    result = run(command, quiet=True)
    return result.stdout.strip() if result.returncode == 0 else ""


def gather(tag: str) -> State:
    if run(["git", "fetch", "--quiet", "--tags", "origin", "main"], quiet=True).returncode != 0:
        raise Refused("origin could not be reached to compare with main")

    head = output(["git", "rev-parse", "HEAD"])
    remote_tag = output(["git", "ls-remote", "--tags", "origin", f"refs/tags/{tag}^{{}}"]) \
        or output(["git", "ls-remote", "--tags", "origin", f"refs/tags/{tag}"])
    local_tag = output(["git", "rev-parse", "--verify", "--quiet", f"refs/tags/{tag}^{{commit}}"])
    tag_commit = remote_tag.split()[0] if remote_tag else (local_tag or None)

    try:
        notes_found = bool(release_notes.notes_for(tag, allow_unreleased=False))
    except SystemExit:
        notes_found = False

    return State(
        clean=output(["git", "status", "--porcelain"]) == "",
        head=head,
        head_on_main=run(["git", "merge-base", "--is-ancestor", "HEAD", "origin/main"], quiet=True).returncode == 0,
        notes_found=notes_found,
        tag_commit=tag_commit,
        release_exists=run(["gh", "release", "view", tag], quiet=True).returncode == 0,
        gh_logged_in=run(["gh", "auth", "status"], quiet=True).returncode == 0,
        inno_setup=local_check.inno_setup() is not None,
    )


def remove_build() -> None:
    run(["git", "worktree", "remove", "--force", str(BUILD)], quiet=True)
    if BUILD.exists():
        shutil.rmtree(BUILD, ignore_errors=True)
    run(["git", "worktree", "prune"], quiet=True)


def build(tag: str, head: str) -> dict:
    """Run every check and the release dry run in a fresh worktree of the commit."""
    remove_build()
    if run(["git", "worktree", "add", "--detach", str(BUILD), head]).returncode != 0:
        raise Refused("a fresh copy of the commit could not be created")

    # The pinned tools are downloaded once per PC, not once per release.
    tools = local_check.TOOLS_DIR
    if tools.exists():
        shutil.copytree(tools, BUILD / ".local-testing" / "tools", dirs_exist_ok=True)

    env, _ = local_check.clean_environment(dict(os.environ))
    local_check.prepare_path(env, local_check.local_tools())
    checks = run([sys.executable, str(BUILD / "scripts" / "local_check.py"), "--release", "--version", tag],
                 cwd=BUILD, env=env)

    report_file = BUILD / ".local-testing" / "local-check.json"
    if not report_file.exists():
        raise Refused("the checks left no report, so nothing was published")
    report = json.loads(report_file.read_text(encoding="utf-8"))
    if checks.returncode != 0 and not failed_checks(report, publish=False):
        raise Refused("the checks did not finish, so nothing was published")
    return report


def collect(tag: str) -> Path:
    """Keep the files outside the worktree, which is removed afterwards."""
    source = BUILD / ".local-testing" / "release"
    target = local_check.STATE / f"release-{tag}"
    if target.exists():
        shutil.rmtree(target)
    target.mkdir(parents=True)
    for name in (*ASSETS, "notes.md"):
        if (source / name).is_file():
            shutil.copy2(source / name, target / name)
    return target


def publish(tag: str, files: Path, tag_commit: str | None) -> str:
    if tag_commit is None:
        if run(["git", "tag", "-a", tag, "-m", f"AUVC {tag}", "HEAD"]).returncode != 0:
            raise Refused(f"the tag {tag} could not be created")
    if run(["git", "push", "origin", f"refs/tags/{tag}"]).returncode != 0:
        raise Refused(f"the tag {tag} could not be pushed; nothing was uploaded")

    created = run(["gh", "release", "create", tag, *(str(files / name) for name in ASSETS),
                   "--verify-tag", "--prerelease", "--title", f"AUVC {tag}",
                   "--notes-file", str(files / "notes.md")])
    if created.returncode != 0:
        raise Refused(f"the upload failed after the tag was pushed. Run the same command again "
                      f"to finish the release {tag}")
    return output(["gh", "release", "view", tag, "--json", "url", "--jq", ".url"])


def main(argv: list[str] | None = None) -> int:
    for stream in (sys.stdout, sys.stderr):
        with contextlib.suppress(AttributeError, ValueError):
            stream.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description="Build a pre-release on this computer and publish it.")
    parser.add_argument("version", help="the pre-release, for example v1.2.3-beta or v1.2.3-rc.1")
    parser.add_argument("--publish", action="store_true",
                        help="tag the commit, push the tag and upload the release after every check passed")
    arguments = parser.parse_args(argv)
    tag = arguments.version

    try:
        check_version(tag)
        state = gather(tag)
        reasons = refusals(tag, state, arguments.publish)
        if reasons:
            raise Refused("\n  - ".join(["nothing was built:", *reasons]))
        if not state.inno_setup:
            print(f"Inno Setup 6 is not installed, so this build has no installer. "
                  f"Publishing needs it: {INNO_SETUP_INSTALL}", flush=True)

        try:
            report = build(tag, state.head)
            problems = failed_checks(report, arguments.publish)
            files = collect(tag)
        finally:
            remove_build()

        if problems:
            raise Refused("\n  - ".join(["checks failed, so nothing was published:", *problems]))
        missing = missing_assets(files) if arguments.publish else []
        sums = (files / "SHA256SUMS").read_text(encoding="utf-8") if (files / "SHA256SUMS").exists() else ""
        unlisted = unlisted_in_checksums(sums) if arguments.publish else []
        if missing or unlisted:
            raise Refused("the release is incomplete: " + ", ".join(
                [f"{name} is missing" for name in missing] + [f"{name} is not in SHA256SUMS" for name in unlisted]))

        if not arguments.publish:
            print(f"\n{tag} is built and every check passed. Nothing was published.\nFiles: {files}\n"
                  f"Publish it with: python scripts/local_release.py {tag} --publish")
            return 0

        url = publish(tag, files, state.tag_commit)
        print(f"\nPublished the pre-release {tag}: {url}")
        return 0
    except Refused as reason:
        print(f"\n{tag}: {reason}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
