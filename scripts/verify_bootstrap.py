#!/usr/bin/env python3
"""Check the import contract and the scaffold of today's repository (#194).

The phase-1 version compared HEAD:bot and HEAD:capture with the upstream trees,
which stopped being true with the first AUVC commit in either folder, and asked
for four documents deleted on 2026-09-14. The import trees are now checked at
their import commits, as scripts/verify_repository.py does, and the scaffold
list holds what the repository has today. On top of verify_repository.py this
checks that the scaffold is complete and that every new file is UTF-8 text.
"""

from pathlib import Path
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
import verify_repository  # noqa: E402

ROOT = verify_repository.ROOT
REQUIRED = (
    ".github/workflows/README.md",
    ".github/ISSUE_TEMPLATE/bug_report.yml",
    ".github/ISSUE_TEMPLATE/feature_request.yml",
    ".github/PULL_REQUEST_TEMPLATE.md",
    "protocol/README.md",
    "docs/acceptance.md",
    "docs/architecture.md",
    "docs/development.md",
    "docs/guide.md",
    "docs/anleitung.md",
    "docs/privacy.md",
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


def check() -> list[str]:
    failures = list(verify_repository.check())
    for path in REQUIRED:
        if not (ROOT / path).is_file():
            failures.append(f"Missing required file: {path}")

    paths = verify_repository.git("ls-files", "-z", "--cached", "--others", "--exclude-standard")
    for raw in sorted(set(paths.split(b"\0")) - {b""}):
        relative = raw.decode("utf-8")
        if relative.startswith(("bot/", "capture/", "LICENSES/")):
            continue
        path = ROOT / relative
        if not path.is_file():
            failures.append(f"Missing tracked file: {relative}")
            continue
        try:
            path.read_bytes().decode("utf-8")
        except UnicodeDecodeError:
            failures.append(f"Not UTF-8 text: {relative}")
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
    print("PASS: import trees at their import commits, licences, scaffold, links and text format")
    print("Run Gitleaks separately; this verifier is not an application test suite.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
