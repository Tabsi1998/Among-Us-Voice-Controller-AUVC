#!/usr/bin/env python3
"""Run every check the CI runs, on this computer.

The CI is the second, independent confirmation. This is the first: the checks
of .github/workflows/baseline.yml, run with the developer's own tools and
processor, plus an optional release dry run that publishes nothing.

Groups:
    repository  provenance, upstream references, the tests of these scripts,
                Gitleaks over the history and over uncommitted and new files
    bot         gofmt, go vet, go test -race, go build, the Windows build of the
                bot, govulncheck
    capture     locked restore, format, build, tests, the self-contained
                publish, vulnerable NuGet packages (Windows only)
    release     release notes, the app with the bot inside, the portable zip,
                SHA256SUMS, and the installer when Inno Setup 6 is installed

Usage:
    python scripts/local_check.py                repository, bot and capture
    python scripts/local_check.py --only bot     some groups
    python scripts/local_check.py --release      all of them and a release dry run
    python scripts/local_check.py --list         the steps, without running them

Results go to .local-testing/, which Git ignores.
"""

from __future__ import annotations

import argparse
import contextlib
import hashlib
import io
import json
import os
import platform
import re
import shutil
import subprocess
import sys
import tarfile
import time
import urllib.request
import xml.etree.ElementTree as ET
import zipfile
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable, Iterable, Iterator

import release_notes

ROOT = Path(__file__).resolve().parents[1]
BOT = ROOT / "bot"
CAPTURE = ROOT / "capture"
CHANGELOG = ROOT / "CHANGELOG.md"
SOLUTION = "AmongUsCapture.sln"

STATE = ROOT / ".local-testing"
TOOLS_DIR = STATE / "tools"
PUBLISH_DIR = STATE / "publish" / "win-x64"
RELEASE_DIR = STATE / "release"

GITLEAKS_VERSION = "8.30.1"
# The official release archives and their published SHA-256. The Linux entry is
# the same pin baseline.yml downloads; a test keeps the two in step.
GITLEAKS_BUILDS = {
    ("Windows", "amd64"): (
        "gitleaks_8.30.1_windows_x64.zip",
        "d29144deff3a68aa93ced33dddf84b7fdc26070add4aa0f4513094c8332afc4e",
    ),
    ("Linux", "x86_64"): (
        "gitleaks_8.30.1_linux_x64.tar.gz",
        "551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb",
    ),
}
GOVULNCHECK = "golang.org/x/vuln/cmd/govulncheck@v1.8.0"

# The CI fails when fewer capture tests run than this, so tests that silently
# stop being discovered cannot pass as green.
MIN_CAPTURE_TESTS = 99
PAYLOAD_FILES = ("hostfxr.dll", "coreclr.dll", "PresentationFramework.dll")
LOCAL_VERSION = "v0.0.0-local"

GROUPS = ("repository", "bot", "capture", "release")
DEFAULT_GROUPS = ("repository", "bot", "capture")

PASS, FAIL, SKIP = "passed", "failed", "skipped"

# Variables whose names look like credentials. None of the checks needs one, and
# a shell or a test setup that happens to carry a real bot token or a database
# password must not hand it to code under test.
SECRET_NAME = re.compile(
    r"TOKEN|SECRET|PASSW|CREDENTIAL|PRIVATE|API_?KEY|ENCRYPTION|_KEY$"
    r"|^(MONGO|SMTP|STRIPE|RESEND|JWT|DISCORD|ADMIN)_",
    re.IGNORECASE,
)


class StepFailed(Exception):
    """A step found a problem. The message says what, and how to fix it."""


class StepSkipped(Exception):
    """A step cannot run here. The message says why."""


@dataclass
class Step:
    group: str
    name: str
    describe: str
    action: Callable[["Context"], str | None]
    needs: tuple[str, ...] = ()


@dataclass
class Result:
    group: str
    name: str
    describe: str
    status: str
    seconds: float
    detail: str = ""


def clean_environment(source: dict[str, str]) -> tuple[dict[str, str], list[str]]:
    """Split an environment into what the steps get and the names withheld."""
    kept: dict[str, str] = {}
    withheld: list[str] = []
    for name, value in source.items():
        if SECRET_NAME.search(name):
            withheld.append(name)
        else:
            kept[name] = value
    return kept, sorted(withheld)


def local_tools() -> dict[str, str]:
    """The tool paths of the VS Code test setup on this PC, when there is one.

    Only the "tools" pointer is read from .vscode/testing.json; nothing else in
    that file is used or printed.
    """
    try:
        config = json.loads((ROOT / ".vscode" / "testing.json").read_text(encoding="utf-8"))
        return json.loads(Path(config["tools"]).read_text(encoding="utf-8"))
    except (OSError, KeyError, TypeError, ValueError):
        return {}


def prepare_path(env: dict[str, str], tools: dict[str, str]) -> None:
    """Put the pinned local tools, and the tools this script installs, first."""
    extra = [tools[key] for key in ("go", "dotnet") if tools.get(key)]
    if tools.get("clang"):
        extra.append(str(Path(tools["clang"]).parent))
    extra.append(str(TOOLS_DIR / "bin"))
    env["PATH"] = os.pathsep.join(extra + [env.get("PATH", "")])
    if tools.get("dotnet"):
        env.setdefault("DOTNET_ROOT", tools["dotnet"])
    env.setdefault("DOTNET_CLI_TELEMETRY_OPTOUT", "1")
    env.setdefault("DOTNET_NOLOGO", "1")


@dataclass
class Context:
    env: dict[str, str]
    log: io.TextIOBase
    version: str
    passed: set[str] = field(default_factory=set)

    def tool(self, name: str) -> str | None:
        return shutil.which(name, path=self.env.get("PATH"))

    def run(
        self,
        command: Iterable[object],
        cwd: Path = ROOT,
        env: dict[str, str] | None = None,
        echo: bool = True,
        lines: Callable[[str], str] | None = None,
    ) -> tuple[int, str]:
        """Run a command, show and log its output, and return code and output."""
        argv = [str(part) for part in command]
        shown = "> " + subprocess.list2cmdline(argv)
        print(shown, flush=True)
        self.log.write(shown + "\n")
        try:
            child = subprocess.Popen(
                argv, cwd=cwd, env={**self.env, **(env or {})},
                stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                text=True, encoding="utf-8", errors="replace",
            )
        except OSError as error:
            raise StepFailed(f"{argv[0]} could not be started: {error}") from error

        captured: list[str] = []
        with child:
            assert child.stdout is not None
            for line in child.stdout:
                self.log.write(line)
                captured.append(line)
                visible = lines(line) if lines else line
                if echo and visible:
                    print(visible, end="", flush=True)
        return child.returncode, "".join(captured)

    def require(self, command: Iterable[object], what: str, cwd: Path = ROOT,
                env: dict[str, str] | None = None) -> str:
        code, output = self.run(command, cwd, env)
        if code != 0:
            raise StepFailed(what)
        return output


def command_step(tool: Callable[[Context], str], what: str, *arguments: str,
                 cwd: Path = ROOT) -> Callable[[Context], None]:
    """A step that runs one command and needs nothing from its output.

    The output is shown and logged while it runs; it is not the step's result,
    which would put a whole build log into the summary.
    """
    def action(context: Context) -> None:
        context.require([tool(context), *arguments], cwd=cwd, what=what)
    return action


def execute(steps: list[Step], context: Context) -> list[Result]:
    """Run the steps in order. A step whose prerequisite did not pass is skipped."""
    results: list[Result] = []
    status_of: dict[str, str] = {}

    for step in steps:
        started = time.monotonic()
        blocked = [need for need in step.needs if status_of.get(need) != PASS]
        if blocked:
            status, detail = SKIP, "needs " + ", ".join(blocked) + " to pass first"
        else:
            print(f"\n== {step.group}: {step.describe}", flush=True)
            try:
                detail = step.action(context) or ""
                status = PASS
            except StepSkipped as reason:
                status, detail = SKIP, str(reason)
            except StepFailed as problem:
                status, detail = FAIL, str(problem)

        status_of[step.name] = status
        if status == PASS:
            context.passed.add(step.name)
        results.append(Result(step.group, step.name, step.describe, status,
                              round(time.monotonic() - started, 1), detail))
    return results


# Repository.

def run_script(context: Context, *arguments: str) -> None:
    context.require([sys.executable, *arguments], what="the check failed; its output above says why")


def git(context: Context) -> str:
    found = context.tool("git")
    if not found:
        raise StepFailed("Git is not on PATH")
    return found


def verify_sha256(data: bytes, expected: str, name: str) -> None:
    actual = hashlib.sha256(data).hexdigest()
    if actual != expected:
        raise StepFailed(f"{name} does not match its published SHA-256 (got {actual}), so it was not used")


def extract_member(data: bytes, archive: str, member: str) -> bytes:
    try:
        if archive.endswith(".zip"):
            with zipfile.ZipFile(io.BytesIO(data)) as bundle:
                return bundle.read(member)
        with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as bundle:
            extracted = bundle.extractfile(member)
            if extracted is None:
                raise KeyError(member)
            return extracted.read()
    except (KeyError, zipfile.BadZipFile, tarfile.TarError) as error:
        raise StepFailed(f"{archive} does not contain {member}") from error


def ensure_gitleaks(context: Context) -> str:
    """The pinned Gitleaks, from PATH or downloaded and checksum-verified once."""
    on_path = context.tool("gitleaks")
    if on_path:
        code, output = context.run([on_path, "version"], echo=False)
        if code == 0 and output.strip() == GITLEAKS_VERSION:
            return on_path

    build = GITLEAKS_BUILDS.get((platform.system(), platform.machine().lower()))
    if build is None:
        raise StepFailed(
            f"there is no pinned Gitleaks {GITLEAKS_VERSION} build for {platform.system()} "
            f"{platform.machine()}; put gitleaks {GITLEAKS_VERSION} on PATH")

    archive, digest = build
    member = "gitleaks.exe" if platform.system() == "Windows" else "gitleaks"
    target = TOOLS_DIR / f"gitleaks-{GITLEAKS_VERSION}" / member
    if target.exists():
        return str(target)

    url = f"https://github.com/gitleaks/gitleaks/releases/download/v{GITLEAKS_VERSION}/{archive}"
    print(f"Downloading Gitleaks {GITLEAKS_VERSION} and checking its SHA-256", flush=True)
    try:
        with urllib.request.urlopen(url, timeout=120) as response:
            data = response.read()
    except OSError as error:
        raise StepFailed(f"Gitleaks could not be downloaded: {error}") from error

    verify_sha256(data, digest, archive)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(extract_member(data, archive, member))
    if platform.system() != "Windows":
        target.chmod(0o755)
    return str(target)


def gitleaks_history(context: Context) -> None:
    binary = ensure_gitleaks(context)
    git(context)
    context.require(
        [binary, "git", ".", "--redact", "--config", ".gitleaks.toml", "--log-opts=HEAD", "--no-banner"],
        what="Gitleaks found a possible secret in the history. Rotate it first, then follow SECURITY.md")


def gitleaks_changes(context: Context) -> str:
    binary = ensure_gitleaks(context)
    scan = [binary, "git", ".", "--redact", "--config", ".gitleaks.toml", "--no-banner"]
    what = "Gitleaks found a possible secret in uncommitted changes. Remove it before committing"
    context.require(scan + ["--pre-commit"], what=what)
    context.require(scan + ["--pre-commit", "--staged"], what=what)

    # git diff does not show files Git does not know yet.
    code, output = context.run([git(context), "ls-files", "--others", "--exclude-standard"], echo=False)
    if code != 0:
        raise StepFailed("new files could not be listed")
    new = [line.strip() for line in output.splitlines() if line.strip()]
    for path in new:
        context.require([binary, "dir", path, "--redact", "--config", ".gitleaks.toml", "--no-banner"],
                        what=f"Gitleaks found a possible secret in {path}. Remove it before committing")
    return f"{len(new)} new file(s) scanned" if new else ""


def repository_steps() -> list[Step]:
    return [
        Step("repository", "provenance", "Provenance, licences and document links",
             lambda context: run_script(context, "scripts/verify_repository.py")),
        Step("repository", "upstream", "No new references to AutoMuteUs infrastructure",
             lambda context: run_script(context, "scripts/check_upstream_references.py")),
        Step("repository", "script-tests", "Tests of the repository scripts",
             lambda context: run_script(context, "-m", "unittest", "discover", "-s", "scripts", "-p", "test_*.py")),
        Step("repository", "gitleaks-history", "Gitleaks over the history", gitleaks_history),
        Step("repository", "gitleaks-changes", "Gitleaks over uncommitted and new files", gitleaks_changes),
    ]


# Bot.

def go(context: Context) -> str:
    found = context.tool("go")
    if not found:
        raise StepFailed("Go is not on PATH. Install Go 1.27.1, or run from the VS Code test setup")
    return found


def unformatted(output: str) -> list[str]:
    return [line.strip() for line in output.splitlines() if line.strip()]


def gofmt(context: Context) -> None:
    binary = context.tool("gofmt")
    if not binary:
        raise StepFailed("gofmt is not on PATH; it comes with Go")
    code, output = context.run([binary, "-l", "."], cwd=BOT)
    if code != 0:
        raise StepFailed("gofmt could not read the code")
    files = unformatted(output)
    if files:
        raise StepFailed("not formatted: " + ", ".join(files) + ". Run gofmt -w on them")


def c_compiler(context: Context) -> str:
    for candidate in (context.env.get("CC"), "gcc", "clang"):
        if candidate and context.tool(candidate):
            return candidate
    raise StepFailed(
        "go test -race needs a C compiler and none was found. "
        "On Windows, put the clang of llvm-mingw on PATH or set CC")


class GoEvents:
    """Keep every go test -json event and show only what a person needs.

    Package results are shown as they come. A test's own output is held back
    and shown only if that test fails, so a green run stays readable and a red
    one shows exactly what broke.
    """

    def __init__(self, sink: io.TextIOBase):
        self.sink = sink
        self.pending: dict[tuple[str, str], list[str]] = {}

    def __call__(self, line: str) -> str:
        self.sink.write(line)
        try:
            event = json.loads(line)
        except ValueError:
            return line
        if not isinstance(event, dict):
            return line

        test = event.get("Test")
        key = (event.get("Package", ""), test or "")
        action = event.get("Action")
        if action == "output":
            if test:
                self.pending.setdefault(key, []).append(event.get("Output", ""))
                return ""
            return event.get("Output", "")
        if test and action in ("pass", "skip"):
            self.pending.pop(key, None)
        elif test and action == "fail":
            return "".join(self.pending.pop(key, []))
        return ""


def go_tests(context: Context) -> None:
    compiler = c_compiler(context)
    STATE.mkdir(exist_ok=True)
    with (STATE / "go-events.jsonl").open("w", encoding="utf-8") as events:
        code, _ = context.run(
            [go(context), "test", "-race", "-count=1", "-json",
             "-coverprofile=" + str(STATE / "go-coverage.out"), "./..."],
            cwd=BOT, env={"CGO_ENABLED": "1", "CC": compiler}, lines=GoEvents(events))
    if code != 0:
        raise StepFailed("Go tests failed; the failing tests are shown above")


def windows_build(context: Context) -> None:
    target = STATE / "build" / "auvc.exe"
    target.parent.mkdir(parents=True, exist_ok=True)
    context.require(
        [go(context), "build", "-trimpath", "-o", target, "."], cwd=BOT,
        env={"CGO_ENABLED": "0", "GOOS": "windows", "GOARCH": "amd64"},
        what="the bot does not build for Windows without cgo, which is how the app ships it")


def govulncheck(context: Context) -> None:
    if not context.tool("govulncheck"):
        (TOOLS_DIR / "bin").mkdir(parents=True, exist_ok=True)
        context.require([go(context), "install", GOVULNCHECK], cwd=BOT,
                        env={"GOBIN": str(TOOLS_DIR / "bin"), "CGO_ENABLED": "0"},
                        what="govulncheck could not be installed")
    context.require([sys.executable, "scripts/check_go_vulnerabilities.py"],
                    what="govulncheck reported a vulnerability that has not been assessed; see above")


def bot_steps() -> list[Step]:
    return [
        Step("bot", "gofmt", "Go formatting", gofmt),
        Step("bot", "vet", "go vet", command_step(go, "go vet reported problems", "vet", "./...", cwd=BOT)),
        Step("bot", "race", "Go tests with the race detector", go_tests),
        Step("bot", "build", "go build", command_step(go, "the bot does not build", "build", "./...", cwd=BOT)),
        Step("bot", "windows", "The bot as the Windows app ships it", windows_build),
        Step("bot", "govulncheck", "Known Go vulnerabilities", govulncheck),
    ]


# Capture.

def dotnet(context: Context) -> str:
    if platform.system() != "Windows":
        raise StepSkipped("the app is a Windows program; run this group on Windows")
    found = context.tool("dotnet")
    if not found:
        raise StepFailed("dotnet is not on PATH. Install the SDK from capture/global.json")
    return found


def trx_counts(text: str) -> tuple[int, int]:
    """Executed and failed tests from a .trx report."""
    root = ET.fromstring(text.lstrip("﻿"))
    for element in root.iter():
        if element.tag.rsplit("}", 1)[-1] == "Counters":
            return int(element.get("executed", 0)), int(element.get("failed", 0))
    raise StepFailed("the test report has no counters")


def capture_tests(context: Context) -> str:
    results = STATE / "dotnet"
    report = results / "local.trx"
    report.unlink(missing_ok=True)
    code, _ = context.run(
        [dotnet(context), "test", "AUVC.Capture.Tests/AUVC.Capture.Tests.csproj",
         "--configuration", "Release", "--no-build", "--no-restore",
         "--logger", "trx;LogFileName=local.trx", "--results-directory", results],
        cwd=CAPTURE)
    if not report.exists():
        raise StepFailed("the tests left no report")

    executed, failed = trx_counts(report.read_text(encoding="utf-8"))
    if code != 0 or failed:
        raise StepFailed(f"{failed} of {executed} tests failed")
    if executed < MIN_CAPTURE_TESTS:
        raise StepFailed(f"only {executed} tests ran, fewer than the {MIN_CAPTURE_TESTS} the CI requires; "
                         "tests were lost or are no longer discovered")
    return f"{executed} tests passed"


def lock_files() -> list[Path]:
    return sorted(path for path in CAPTURE.rglob("packages.lock.json")
                  if not {"bin", "obj"} & set(path.relative_to(CAPTURE).parts))


@contextlib.contextmanager
def restored_afterwards(paths: Iterable[Path]) -> Iterator[None]:
    """Put files back exactly as they were, around a command known to rewrite them."""
    before = {path: path.read_bytes() for path in paths if path.exists()}
    try:
        yield
    finally:
        for path, content in before.items():
            if not path.exists() or path.read_bytes() != content:
                path.write_bytes(content)


def payload_problems(folder: Path) -> list[str]:
    problems = []
    if not (folder / "AUCapture-WPF.exe").exists():
        problems.append("the publish produced no AUCapture-WPF.exe")
    try:
        config = json.loads((folder / "AUCapture-WPF.runtimeconfig.json").read_text(encoding="utf-8"))
    except (OSError, ValueError):
        problems.append("the publish has no readable AUCapture-WPF.runtimeconfig.json")
    else:
        if not config.get("runtimeOptions", {}).get("includedFrameworks"):
            problems.append("the app is framework-dependent and would need .NET installed on the player's PC")
    for name in PAYLOAD_FILES:
        if not (folder / name).exists():
            problems.append(f"the payload is missing {name}")
    return problems


def publish_app(context: Context) -> None:
    binary = dotnet(context)
    if PUBLISH_DIR.exists():
        shutil.rmtree(PUBLISH_DIR)
    # A publish for a runtime adds that runtime to the lock files, which would
    # fail the next locked restore. The files go back exactly as they were.
    with restored_afterwards(lock_files()):
        context.require(
            [binary, "publish", "AUCapture-WPF/AUCapture-WPF.csproj", "--configuration", "Release",
             "--runtime", "win-x64", "--self-contained", "true", "-o", PUBLISH_DIR],
            cwd=CAPTURE, what="the app did not publish")
    problems = payload_problems(PUBLISH_DIR)
    if problems:
        raise StepFailed("; ".join(problems))


def vulnerable_packages(report: str) -> list[str]:
    """Rows of `dotnet list package --vulnerable`.

    Vulnerable packages are listed as "> name version severity" rows. The prose
    around them is localized; this marker is not.
    """
    return [line.strip()[1:].strip() for line in report.splitlines() if re.match(r"^\s*>\s", line)]


def nuget_vulnerabilities(context: Context) -> None:
    code, output = context.run(
        [dotnet(context), "list", SOLUTION, "package", "--vulnerable", "--include-transitive"], cwd=CAPTURE)
    if code != 0:
        raise StepFailed("the vulnerability list could not be read; it needs access to nuget.org")
    found = vulnerable_packages(output)
    if found:
        raise StepFailed("vulnerable NuGet packages: " + "; ".join(found))


def capture_steps() -> list[Step]:
    def dotnet_step(what: str, *arguments: str) -> Callable[[Context], None]:
        return command_step(dotnet, what, *arguments, cwd=CAPTURE)

    return [
        Step("capture", "restore", "Locked NuGet restore",
             dotnet_step("the restore does not match packages.lock.json; see docs/development.md",
                         "restore", SOLUTION, "--locked-mode")),
        Step("capture", "format", "C# whitespace formatting",
             dotnet_step("C# files are not formatted; run dotnet format whitespace",
                         "format", "whitespace", SOLUTION, "--verify-no-changes", "--no-restore"),
             needs=("restore",)),
        Step("capture", "dotnet-build", "Release build",
             dotnet_step("the app does not build", "build", SOLUTION, "--configuration", "Release", "--no-restore"),
             needs=("restore",)),
        Step("capture", "dotnet-tests", f"C# tests (at least {MIN_CAPTURE_TESTS} must run)", capture_tests,
             needs=("dotnet-build",)),
        Step("capture", "publish", "Self-contained app for PCs without .NET", publish_app, needs=("restore",)),
        Step("capture", "nuget-vulnerabilities", "Vulnerable NuGet packages", nuget_vulnerabilities,
             needs=("restore",)),
    ]


# Release dry run.

def default_version(changelog: str) -> str:
    """What a dry run previews: the unreleased changes if there are any, else the newest release."""
    found = release_notes.sections(changelog)
    for heading, body in found.items():
        if heading.strip().lower() == "unreleased" and body:
            return LOCAL_VERSION
    for heading in found:
        version = release_notes.normalise(heading)
        if re.match(r"^\d+\.\d+\.\d+", version):
            return "v" + version
    return LOCAL_VERSION


def notes(context: Context) -> str:
    if RELEASE_DIR.exists():
        shutil.rmtree(RELEASE_DIR)
    RELEASE_DIR.mkdir(parents=True)
    code, output = context.run([sys.executable, "scripts/release_notes.py", context.version, "--allow-unreleased"],
                               echo=False, env={"PYTHONIOENCODING": "utf-8"})
    if code != 0:
        raise StepFailed(output.strip() or "there are no release notes")
    (RELEASE_DIR / "notes.md").write_text(output, encoding="utf-8")
    return f"{len(output.splitlines())} lines for {context.version}"


def release_payload(context: Context) -> None:
    if "publish" not in context.passed:
        publish_app(context)

    for name in ("LICENSE", "THIRD_PARTY_NOTICES.md"):
        shutil.copy2(ROOT / name, PUBLISH_DIR / name)
    shutil.copytree(ROOT / "LICENSES", PUBLISH_DIR / "LICENSES", dirs_exist_ok=True)

    code, commit = context.run([git(context), "rev-parse", "HEAD"], echo=False)
    bot_dir = PUBLISH_DIR / "bot"
    bot_dir.mkdir(exist_ok=True)
    context.require(
        [go(context), "build", "-trimpath",
         "-ldflags", f"-s -w -X main.version={context.version} -X main.commit={commit.strip() if code == 0 else 'unknown'}",
         "-o", bot_dir / "auvc.exe", "."],
        cwd=BOT, env={"CGO_ENABLED": "0", "GOOS": "windows", "GOARCH": "amd64"},
        what="the bot did not build for the app")
    shutil.copytree(BOT / "locales", bot_dir / "locales", dirs_exist_ok=True)


def portable_zip(context: Context) -> str:
    target = RELEASE_DIR / "AmongUsVoiceCapture-win-x64.zip"
    target.unlink(missing_ok=True)
    with zipfile.ZipFile(target, "w", zipfile.ZIP_DEFLATED) as bundle:
        for path in sorted(PUBLISH_DIR.rglob("*")):
            if path.is_file():
                bundle.write(path, path.relative_to(PUBLISH_DIR).as_posix())
    return f"{target.stat().st_size / 1_000_000:.0f} MB"


def inno_setup() -> str | None:
    on_path = shutil.which("ISCC")
    if on_path:
        return on_path
    for base in (os.environ.get("ProgramFiles(x86)"), os.environ.get("ProgramFiles")):
        if base and (Path(base) / "Inno Setup 6" / "ISCC.exe").exists():
            return str(Path(base) / "Inno Setup 6" / "ISCC.exe")
    return None


def installer(context: Context) -> None:
    iscc = inno_setup()
    if not iscc:
        raise StepSkipped("Inno Setup 6 is not installed; the release workflow builds the installer")
    version = context.version.removeprefix("v")
    if not re.match(r"^\d+\.\d+\.\d+", version):
        version = "0.0.0"
    context.require([iscc, f"/DAppVersion={version}", f"/DPayloadDir={PUBLISH_DIR}", f"/DOutputDir={RELEASE_DIR}",
                     "auvc-capture.iss"], cwd=ROOT / "installer", what="the installer did not build")


def checksums(context: Context) -> str:
    lines = [f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}"
             for path in sorted(RELEASE_DIR.iterdir()) if path.is_file() and path.suffix in (".zip", ".exe")]
    (RELEASE_DIR / "SHA256SUMS").write_text("\n".join(lines) + "\n", encoding="utf-8")
    return f"{len(lines)} file(s)"


def release_steps() -> list[Step]:
    return [
        Step("release", "notes", "Release notes from CHANGELOG.md", notes),
        Step("release", "payload", "The app with the bot inside", release_payload),
        Step("release", "zip", "Portable zip", portable_zip, needs=("payload",)),
        Step("release", "installer", "Installer (needs Inno Setup 6)", installer, needs=("payload",)),
        Step("release", "checksums", "SHA256SUMS", checksums, needs=("zip",)),
    ]


# Running.

def selected_groups(only: str | None, release: bool) -> set[str]:
    if only:
        chosen = {group.strip() for group in only.split(",") if group.strip()}
        unknown = sorted(chosen - set(GROUPS))
        if unknown:
            raise ValueError("unknown group(s): " + ", ".join(unknown) + "; choose from " + ", ".join(GROUPS))
    else:
        chosen = set(DEFAULT_GROUPS)
    if release:
        chosen.add("release")
    return chosen


def plan(groups: set[str]) -> list[Step]:
    builders = {"repository": repository_steps, "bot": bot_steps, "capture": capture_steps, "release": release_steps}
    return [step for group in GROUPS if group in groups for step in builders[group]()]


def summary(results: list[Result], seconds: float) -> str:
    counts = {status: sum(result.status == status for result in results) for status in (PASS, SKIP, FAIL)}
    lines = [f"\nLocal check: {counts[PASS]} passed, {counts[SKIP]} skipped, {counts[FAIL]} failed "
             f"in {seconds:.0f} s"]
    for result in results:
        mark = {PASS: "PASS", SKIP: "SKIP", FAIL: "FAIL"}[result.status]
        line = f"  {mark}  {result.group:<10}  {result.describe:<48} {result.seconds:>6.1f} s"
        if result.detail:
            line += f"  {result.detail}"
        lines.append(line)
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    for stream in (sys.stdout, sys.stderr):
        with contextlib.suppress(AttributeError, ValueError):
            stream.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description="Run every check the CI runs, on this computer.")
    parser.add_argument("--only", help="comma-separated groups: " + ", ".join(GROUPS))
    parser.add_argument("--release", action="store_true", help="add a release dry run; nothing is published")
    parser.add_argument("--version", help="the version a release dry run builds, for example v0.1.2-beta")
    parser.add_argument("--list", action="store_true", help="show the steps without running them")
    arguments = parser.parse_args(argv)

    try:
        groups = selected_groups(arguments.only, arguments.release)
    except ValueError as error:
        parser.error(str(error))
    steps = plan(groups)

    if arguments.list:
        for step in steps:
            print(f"{step.group:<11} {step.describe}")
        return 0

    env, withheld = clean_environment(dict(os.environ))
    prepare_path(env, local_tools())
    if withheld:
        print("Withheld from every step (names only): " + ", ".join(withheld), flush=True)

    STATE.mkdir(exist_ok=True)
    version = arguments.version or default_version(CHANGELOG.read_text(encoding="utf-8"))
    started = time.monotonic()
    with (STATE / "local-check.log").open("w", encoding="utf-8") as log:
        results = execute(steps, Context(env=env, log=log, version=version))
    seconds = time.monotonic() - started

    report = {"groups": sorted(groups), "seconds": round(seconds, 1),
              "results": [result.__dict__ for result in results]}
    (STATE / "local-check.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
    print(summary(results, seconds))
    print(f"Report: {STATE / 'local-check.json'}")
    return 1 if any(result.status == FAIL for result in results) else 0


if __name__ == "__main__":
    sys.exit(main())
