#!/usr/bin/env python3
"""Verify preserved import history, licensing and current repository documents."""

from pathlib import Path
import re
import subprocess
import sys
import tomllib

ROOT = Path(__file__).resolve().parents[1]
IMPORTS = (
    ("bot", "AutoMuteUs", "772c0872d252081a19b702dad7c4a9a673432e82",
     "d9ae52ffc0e804e0211154ace579425fc5ef9f4f"),
    ("capture", "AmongUsCapture", "197c2a9a4242351ba13279d199685802c5506bca",
     "d05d7cd8f300b4368d3872337d2a9f5059c76ee6"),
)


def git(*args: str) -> bytes:
    return subprocess.check_output(["git", "-C", str(ROOT), *args])


def check() -> list[str]:
    errors = []
    for prefix, name, commit, tree in IMPORTS:
        git("merge-base", "--is-ancestor", commit, "HEAD")
        if git("rev-parse", f"{commit}:{prefix}").decode().strip() != tree:
            errors.append(f"{prefix}: original import tree identity changed")
        license_bytes = git("show", f"{commit}:{prefix}/LICENSE")
        for file in (f"{prefix}/LICENSE", f"LICENSES/{name}-MIT.txt"):
            if (ROOT / file).read_bytes() != license_bytes:
                errors.append(f"{file}: original license bytes changed")
    if "Copyright (c) 2026 IT-Tabelander" not in (ROOT / "LICENSE").read_text():
        errors.append("AUVC copyright missing")
    readme = (ROOT / "README.md").read_text(encoding="utf-8")
    if "## Upstream and Credits" not in readme or "not an official AutoMuteUs project" not in readme:
        errors.append("Independent-project statement or credits missing")
    config = tomllib.loads((ROOT / ".gitleaks.toml").read_text(encoding="utf-8"))
    if config.get("extend", {}).get("useDefault") is not True:
        errors.append("Default Gitleaks detectors disabled")
    for path in (
        ".github/workflows/baseline.yml", ".github/workflows/release.yml",
        "capture/global.json", "capture/AUVC.Capture.Tests/AUVC.Capture.Tests.csproj",
        "installer/auvc-capture.iss",
        "docs/guide.md", "docs/anleitung.md", "docs/architecture.md",
        "docs/development.md", "docs/privacy.md",
        "UPSTREAM.md", "SECURITY.md", "THIRD_PARTY_NOTICES.md",
    ):
        if not (ROOT / path).is_file():
            errors.append(f"Missing required file: {path}")
    paths = git("ls-files", "-z", "--cached", "--others", "--exclude-standard")
    for raw in sorted(set(paths.split(b"\0")) - {b""}):
        relative = raw.decode()
        if relative.startswith(("bot/", "capture/", "LICENSES/")):
            continue
        path = ROOT / relative
        data = path.read_bytes()
        content = data.decode("utf-8")
        if b"\r" in data or (data and not data.endswith(b"\n")):
            errors.append(f"{relative}: expected LF and final newline")
        if any(line.rstrip(" \t") != line for line in content.splitlines()):
            errors.append(f"{relative}: trailing whitespace")
        if path.name.startswith(".env") and path.name not in (".env.example", ".env.sample"):
            errors.append(f"{relative}: runtime environment file tracked")
        if path.suffix in (".key", ".pem", ".pfx", ".p12", ".db", ".sqlite"):
            errors.append(f"{relative}: credential/runtime file tracked")
        if path.suffix == ".md":
            for target in re.findall(r"\[[^\]]*\]\(([^)]+)\)", content):
                if "://" in target or target.startswith("#"):
                    continue
                target = target.split("#", 1)[0]
                if target and not (path.parent / target).exists():
                    errors.append(f"{relative}: broken local link {target}")
    return errors


if __name__ == "__main__":
    try:
        findings = check()
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"FAIL: cannot verify repository: {error}")
        sys.exit(1)
    for finding in findings:
        print(f"FAIL: {finding}")
    if findings:
        sys.exit(1)
    print("PASS: original import history, licenses, repository structure and document format/links")
