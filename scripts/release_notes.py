#!/usr/bin/env python3
"""Print the CHANGELOG section for one release.

The notes people read come from the file the project already keeps by hand.
Generating them from commit subjects instead would produce an accurate list of
what changed and a poor answer to what it means, which is what a reader of
release notes is actually asking.

A release whose version has no section fails rather than shipping empty notes:
writing them is part of preparing a release, and a silent fallback is how that
step gets skipped forever.

Usage:
    python scripts/release_notes.py v1.2.3
    python scripts/release_notes.py v1.2.3 --allow-unreleased
"""

import argparse
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CHANGELOG = ROOT / "CHANGELOG.md"


def sections(text: str) -> dict[str, str]:
    """Split the changelog into its level-two sections, keyed by heading."""
    found: dict[str, str] = {}
    current: str | None = None
    body: list[str] = []

    for line in text.splitlines():
        if line.startswith("## "):
            if current is not None:
                found[current] = "\n".join(body).strip()
            current = line[3:].strip()
            body = []
            continue
        if current is not None:
            body.append(line)

    if current is not None:
        found[current] = "\n".join(body).strip()
    return found


def normalise(heading: str) -> str:
    """Reduce a heading to the version it names.

    Headings are written in several shapes over a project's life — "1.2.3",
    "v1.2.3", "[1.2.3] - 2026-01-01" — and a release should not fail because of
    the brackets somebody used.
    """
    match = re.search(r"v?(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)", heading)
    return match.group(1) if match else heading.strip().lower()


def notes_for(tag: str, allow_unreleased: bool) -> str:
    text = CHANGELOG.read_text(encoding="utf-8")
    found = sections(text)
    wanted = normalise(tag)

    for heading, body in found.items():
        if normalise(heading) == wanted and body:
            return body

    if allow_unreleased:
        for heading, body in found.items():
            if heading.strip().lower() == "unreleased" and body:
                return body

    available = ", ".join(found) or "none"
    print(f"CHANGELOG.md has no section for {tag}.", file=sys.stderr)
    print(f"Sections present: {available}", file=sys.stderr)
    print(file=sys.stderr)
    print("Rename the Unreleased heading to the version being released, so the", file=sys.stderr)
    print("notes say what changed rather than listing commit subjects.", file=sys.stderr)
    raise SystemExit(1)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("tag", help="the release tag, for example v1.2.3")
    parser.add_argument(
        "--allow-unreleased",
        action="store_true",
        help="fall back to the Unreleased section, for a dry run",
    )
    arguments = parser.parse_args()

    print(notes_for(arguments.tag, arguments.allow_unreleased))
    return 0


if __name__ == "__main__":
    sys.exit(main())
