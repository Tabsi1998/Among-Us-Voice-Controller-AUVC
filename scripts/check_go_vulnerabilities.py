#!/usr/bin/env python3
"""Fail when the bot gains a Go vulnerability that has not been assessed.

govulncheck reports vulnerabilities whose vulnerable code the bot actually
calls. A few are known and accepted for now, each recorded below with why and
with what removes it. Anything else fails the build, so a new advisory cannot
slip in unnoticed.

Requires govulncheck on PATH. Install the pinned version with:
    go install golang.org/x/vuln/cmd/govulncheck@v1.8.0

Usage:
    python scripts/check_go_vulnerabilities.py
"""

import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MODULE = ROOT / "bot"

# Vulnerabilities the bot calls into that are accepted despite having no fix.
#
# This map is empty, and that is the point: the two entries it used to hold were
# both in the PostgreSQL layer, and phase 14 removed that layer rather than
# living with them. Nothing reachable from this module has a known advisory.
#
# An entry here needs a reason and the phase that removes it. The check fails on
# anything unlisted, and equally on a listed advisory that no longer applies, so
# the map cannot quietly drift out of step with reality.
ACCEPTED: dict[str, str] = {}


def stream_json(text: str):
    """Yield the JSON objects govulncheck writes, which are pretty-printed and
    therefore cannot be parsed a line at a time."""
    decoder = json.JSONDecoder()
    index = 0
    length = len(text)

    while index < length:
        while index < length and text[index].isspace():
            index += 1
        if index >= length:
            return
        obj, end = decoder.raw_decode(text, index)
        yield obj
        index = end


def govulncheck_binary() -> str:
    """Locate govulncheck.

    PATH is the normal case, but a `go install` puts it in GOPATH/bin, which is
    often not on PATH on a developer machine. Looking there too saves everyone a
    confusing FileNotFoundError.
    """
    found = shutil.which("govulncheck")
    if found:
        return found

    gopath = os.environ.get("GOPATH")
    if not gopath:
        try:
            gopath = subprocess.check_output(["go", "env", "GOPATH"], text=True).strip()
        except (OSError, subprocess.CalledProcessError):
            gopath = ""

    if gopath:
        candidate = shutil.which("govulncheck", path=str(Path(gopath) / "bin"))
        if candidate:
            return candidate

    print("govulncheck was not found. Install the pinned version with:", file=sys.stderr)
    print("  go install golang.org/x/vuln/cmd/govulncheck@v1.8.0", file=sys.stderr)
    raise SystemExit(2)


def called_vulnerabilities() -> dict[str, str]:
    """Return the advisories whose vulnerable functions the bot actually calls,
    mapped to the module they come from."""
    result = subprocess.run(
        [govulncheck_binary(), "-json", "./..."],
        cwd=MODULE,
        capture_output=True,
        text=True,
        # govulncheck emits UTF-8. Without this Python decodes with the console
        # code page on Windows and dies on the first non-ASCII byte.
        encoding="utf-8",
        errors="replace",
    )
    if result.returncode not in (0, 3):
        # 3 means vulnerabilities were found, which is not a tool failure.
        print("govulncheck failed to run:", file=sys.stderr)
        print(result.stderr.strip(), file=sys.stderr)
        raise SystemExit(2)

    modules: dict[str, str] = {}
    called: dict[str, str] = {}

    for message in stream_json(result.stdout):
        finding = message.get("finding")
        if not finding:
            continue

        osv = finding.get("osv")
        trace = finding.get("trace") or []
        if not osv or not trace:
            continue

        frame = trace[0]
        if frame.get("module"):
            modules.setdefault(osv, frame["module"])

        # A finding with a function in its innermost frame means the vulnerable
        # code is reachable, rather than merely present in the module graph.
        if frame.get("function"):
            called[osv] = modules.get(osv, frame.get("module", "unknown"))

    return called


def main() -> int:
    called = called_vulnerabilities()

    unexpected = sorted(osv for osv in called if osv not in ACCEPTED)
    stale = sorted(osv for osv in ACCEPTED if osv not in called)

    if unexpected:
        print("Vulnerabilities the bot calls into that have not been assessed:")
        for osv in unexpected:
            print(f"  {osv} in {called[osv]}")
            print(f"    https://pkg.go.dev/vuln/{osv}")
        print()
        print("Update the dependency if a fix exists. If it does not, record why the")
        print("risk is accepted in ACCEPTED in this script, with the phase that removes it.")

    if stale:
        if unexpected:
            print()
        print("Accepted vulnerabilities that no longer apply:")
        for osv in stale:
            print(f"  {osv}")
        print()
        print("Remove them from ACCEPTED in this script so the list keeps describing reality.")

    if unexpected or stale:
        return 1

    if called:
        print(f"PASS: {len(called)} known and accepted vulnerability(ies), all recorded with the phase that removes them")
    else:
        print("PASS: no vulnerable code is reachable")
    return 0


if __name__ == "__main__":
    sys.exit(main())
