#!/usr/bin/env python3
"""Check the pristine phase-1 import contract using only Python 3.11+ and Git."""

from pathlib import Path
import re
import subprocess
import sys
import tomllib

ROOT = Path(__file__).resolve().parents[1]
BASELINES = (
    ("bot", "AutoMuteUs", "d9ae52ffc0e804e0211154ace579425fc5ef9f4f"),
    ("capture", "AmongUsCapture", "d05d7cd8f300b4368d3872337d2a9f5059c76ee6"),
)
REQUIRED = (
    ".github/workflows/README.md",
    ".github/ISSUE_TEMPLATE/bug_report.yml",
    ".github/ISSUE_TEMPLATE/feature_request.yml",
    ".github/PULL_REQUEST_TEMPLATE.md",
    "protocol/README.md",
    "deploy/README.md",
    "docs/architecture.md",
    "docs/development.md",
    "docs/requirements.md",
    "docs/roadmap.md",
    "docs/bootstrap-validation.md",
    ".editorconfig",
    ".gitattributes",
    ".gitignore",
    ".gitleaks.toml",
    "CHANGELOG.md",
    "LICENSE",
    "README.md",
    "SECURITY.md",
    "THIRD_PARTY_NOTICES.md",
    "UPSTREAM.md",
)


def git(*args: str) -> bytes:
    return subprocess.check_output(
        ["git", "-C", str(ROOT), *args], stderr=subprocess.PIPE
    )


def check() -> list[str]:
    failures = []
    for prefix, name, tree in BASELINES:
        actual = git("rev-parse", f"HEAD:{prefix}").decode().strip()
        if actual != tree:
            failures.append(f"{prefix}: committed tree differs from pinned upstream")
        original = git("show", f"HEAD:{prefix}/LICENSE")
        for path in (f"{prefix}/LICENSE", f"LICENSES/{name}-MIT.txt"):
            if not (ROOT / path).is_file() or (ROOT / path).read_bytes() != original:
                failures.append(f"{path}: differs from original license bytes")
        if b"Copyright (c) 2020 Denver Quane" not in original:
            failures.append(f"{prefix}: expected original copyright missing")

    for args in (("diff", "--exit-code"), ("diff", "--cached", "--exit-code")):
        result = subprocess.run(
            ["git", "-C", str(ROOT), *args, "--", "bot", "capture"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
            check=False,
        )
        if result.returncode:
            failures.append("Imported source has working-tree/index changes")
            break

    for path in REQUIRED:
        if not (ROOT / path).is_file():
            failures.append(f"Missing required file: {path}")

    root_license = (ROOT / "LICENSE").read_text(encoding="utf-8")
    if "Copyright (c) 2026 IT-Tabelander" not in root_license:
        failures.append("Root AUVC copyright missing")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")
    if "## Upstream and Credits" not in readme:
        failures.append("README credits section missing")
    if "not an official AutoMuteUs project" not in readme:
        failures.append("README independent-project statement missing")

    config = tomllib.loads((ROOT / ".gitleaks.toml").read_text(encoding="utf-8"))
    if config.get("extend", {}).get("useDefault") is not True:
        failures.append("Gitleaks default detectors must remain enabled")

    # Include new files so checks work before the first scaffold commit too.
    paths = git("ls-files", "-z", "--cached", "--others", "--exclude-standard")
    for raw in sorted(set(paths.split(b"\0")) - {b""}):
        relative = raw.decode("utf-8")
        if relative.startswith(("bot/", "capture/", "LICENSES/")):
            continue
        path = ROOT / relative
        if not path.is_file():
            failures.append(f"Missing tracked file: {relative}")
            continue
        if path.name.startswith(".env") and path.name not in (
            ".env.example", ".env.sample"
        ):
            failures.append(f"Environment file must not be tracked: {relative}")
        if path.suffix.lower() in {".pem", ".key", ".pfx", ".p12", ".db", ".sqlite"}:
            failures.append(f"Credential/runtime file must not be tracked: {relative}")
        data = path.read_bytes()
        try:
            content = data.decode("utf-8")
        except UnicodeDecodeError:
            failures.append(f"New bootstrap file is not UTF-8 text: {relative}")
            continue
        if b"\r" in data or (data and not data.endswith(b"\n")):
            failures.append(f"Expected LF with final newline: {relative}")
        if any(line.rstrip(" \t") != line for line in content.splitlines()):
            failures.append(f"Trailing whitespace: {relative}")
        if path.suffix == ".md":
            for target in re.findall(r"\[[^\]]*\]\(([^)]+)\)", content):
                if "://" in target or target.startswith("#"):
                    continue
                target = target.split("#", 1)[0]
                if target and not (path.parent / target).exists():
                    failures.append(f"Broken local link in {relative}: {target}")
    return failures


def main() -> int:
    try:
        failures = check()
    except (OSError, subprocess.CalledProcessError, ValueError) as error:
        print(f"FAIL: bootstrap verification could not complete: {error}")
        return 1
    for failure in failures:
        print(f"FAIL: {failure}")
    if failures:
        return 1
    print("PASS: pristine upstream trees, licenses, scaffold, links and text format")
    print("Run Gitleaks separately; this verifier is not an application test suite.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
