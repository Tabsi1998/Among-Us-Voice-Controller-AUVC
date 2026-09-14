"""Tests for scripts/local_check.py.

Run with:
    python -m unittest discover -s scripts -p "test_*.py"
"""

import contextlib
import hashlib
import io
import json
import tempfile
import unittest
from pathlib import Path

import local_check as check


def quietly(function, *arguments):
    with contextlib.redirect_stdout(io.StringIO()):
        return function(*arguments)


class FakeContext:
    def __init__(self):
        self.passed = set()


class EnvironmentTests(unittest.TestCase):
    def test_secret_looking_variables_never_reach_a_step(self):
        kept, withheld = check.clean_environment({
            "PATH": "C:\\tools", "GOPATH": "C:\\go", "DOTNET_ROOT": "C:\\dotnet",
            "SystemRoot": "C:\\Windows", "NUMBER_OF_PROCESSORS": "16", "PYTHONIOENCODING": "utf-8",
            "DISCORD_BOT_TOKEN": "x", "SETTINGS_ENCRYPTION_KEY": "x", "MONGO_URL": "x",
            "JWT_SECRET": "x", "ADMIN_PASSWORD": "x", "ADMIN_EMAIL": "x", "API_ADMIN_TOKEN": "x",
            "RESEND_API_KEY": "x", "STRIPE_SECRET_KEY": "x", "SMTP_HOST": "x", "GITHUB_TOKEN": "x",
            "AUVC_LOCAL_CONTROL_SECRET": "x", "DISCORD_CLIENT_SECRET": "x",
        })

        self.assertEqual(set(kept), {"PATH", "GOPATH", "DOTNET_ROOT", "SystemRoot",
                                     "NUMBER_OF_PROCESSORS", "PYTHONIOENCODING"})
        self.assertEqual(len(withheld), 13)
        self.assertEqual(withheld, sorted(withheld))

    def test_the_pinned_tools_come_first_on_path(self):
        env = {"PATH": "C:\\old-go"}
        check.prepare_path(env, {"go": "C:\\tools\\go\\bin", "dotnet": "C:\\tools\\dotnet",
                                 "clang": "C:\\tools\\llvm\\bin\\clang.exe"})

        entries = env["PATH"].split(check.os.pathsep)
        self.assertEqual(entries[:3], ["C:\\tools\\go\\bin", "C:\\tools\\dotnet", "C:\\tools\\llvm\\bin"])
        self.assertEqual(entries[-1], "C:\\old-go")
        self.assertEqual(env["DOTNET_ROOT"], "C:\\tools\\dotnet")


class GitleaksTests(unittest.TestCase):
    def test_a_download_that_does_not_match_its_checksum_is_refused(self):
        original = hashlib.sha256(b"original").hexdigest()

        with self.assertRaises(check.StepFailed):
            check.verify_sha256(b"tampered", original, "gitleaks.zip")
        check.verify_sha256(b"original", original, "gitleaks.zip")

    def test_the_linux_pin_is_the_one_the_ci_downloads(self):
        workflow = (check.ROOT / ".github" / "workflows" / "baseline.yml").read_text(encoding="utf-8")
        archive, digest = check.GITLEAKS_BUILDS[("Linux", "x86_64")]

        self.assertIn(archive, workflow)
        self.assertIn(digest, workflow)
        self.assertIn("v" + check.GITLEAKS_VERSION, workflow)

    def test_windows_has_a_pinned_build(self):
        archive, digest = check.GITLEAKS_BUILDS[("Windows", "amd64")]

        self.assertTrue(archive.endswith("_windows_x64.zip"))
        self.assertEqual(len(digest), 64)


class ReportTests(unittest.TestCase):
    def test_test_counts_are_read_from_a_trx_report(self):
        report = (
            "\ufeff<?xml version=\"1.0\" encoding=\"utf-8\"?>"
            "<TestRun xmlns=\"http://microsoft.com/schemas/VisualStudio/TeamTest/2010\">"
            "<ResultSummary outcome=\"Completed\">"
            "<Counters total=\"185\" executed=\"184\" passed=\"183\" failed=\"1\" />"
            "</ResultSummary></TestRun>"
        )

        self.assertEqual(check.trx_counts(report), (184, 1))

    def test_a_report_without_counters_fails(self):
        with self.assertRaises(check.StepFailed):
            check.trx_counts("<TestRun />")

    def test_vulnerable_nuget_rows_are_found_in_any_language(self):
        clean = ("Die folgenden Quellen wurden verwendet:\n   https://api.nuget.org/v3/index.json\n\n"
                 "Das Projekt `AUVC.Transport` enthält keine anfälligen Pakete.\n")
        vulnerable = ("Project `AUCapture-WPF` has the following vulnerable packages\n"
                      "   [net10.0-windows7.0]: \n"
                      "   Transitive Package      Resolved   Severity   Advisory URL\n"
                      "   > System.Text.Json      8.0.0      High       https://github.com/advisories/GHSA-x\n")

        self.assertEqual(check.vulnerable_packages(clean), [])
        found = check.vulnerable_packages(vulnerable)
        self.assertEqual(len(found), 1)
        self.assertTrue(found[0].startswith("System.Text.Json"))

    def test_gofmt_output_names_the_unformatted_files(self):
        self.assertEqual(check.unformatted(""), [])
        self.assertEqual(check.unformatted("bot\\a.go\r\n\nbot\\b.go\n"), ["bot\\a.go", "bot\\b.go"])

    def test_go_output_shows_package_results_and_failures_only(self):
        events = [
            {"Action": "run", "Package": "p", "Test": "TestA"},
            {"Action": "output", "Package": "p", "Test": "TestA", "Output": "=== RUN   TestA\n"},
            {"Action": "pass", "Package": "p", "Test": "TestA"},
            {"Action": "output", "Package": "p", "Test": "TestB", "Output": "    b_test.go:9: boom\n"},
            {"Action": "fail", "Package": "p", "Test": "TestB"},
            {"Action": "output", "Package": "p", "Output": "FAIL\tp\t0.1s\n"},
        ]
        sink = io.StringIO()
        reader = check.GoEvents(sink)

        shown = "".join(reader(json.dumps(event) + "\n") for event in events)

        self.assertIn("boom", shown)
        self.assertIn("FAIL\tp", shown)
        self.assertNotIn("=== RUN   TestA", shown)
        self.assertEqual(len(sink.getvalue().splitlines()), len(events))


class PayloadTests(unittest.TestCase):
    def test_files_a_command_rewrites_are_put_back(self):
        with tempfile.TemporaryDirectory() as folder:
            lock = Path(folder) / "packages.lock.json"
            untouched = Path(folder) / "other" / "packages.lock.json"
            untouched.parent.mkdir()
            lock.write_bytes(b'{"version":1}')
            untouched.write_bytes(b'{"version":2}')

            with check.restored_afterwards([lock, untouched]):
                lock.write_bytes(b'{"version":1,"win-x64":{}}')

            self.assertEqual(lock.read_bytes(), b'{"version":1}')
            self.assertEqual(untouched.read_bytes(), b'{"version":2}')

    def test_a_payload_that_needs_installed_dotnet_is_refused(self):
        with tempfile.TemporaryDirectory() as folder:
            payload = Path(folder)
            self.assertEqual(len(check.payload_problems(payload)), 2 + len(check.PAYLOAD_FILES))

            (payload / "AUCapture-WPF.exe").write_bytes(b"MZ")
            for name in check.PAYLOAD_FILES:
                (payload / name).write_bytes(b"MZ")
            config = payload / "AUCapture-WPF.runtimeconfig.json"
            config.write_text('{"runtimeOptions":{"framework":{"name":"Microsoft.WindowsDesktop.App"}}}')
            self.assertEqual(len(check.payload_problems(payload)), 1)

            config.write_text('{"runtimeOptions":{"includedFrameworks":[{"name":"Microsoft.NETCore.App"}]}}')
            self.assertEqual(check.payload_problems(payload), [])


class PlanTests(unittest.TestCase):
    def test_the_release_dry_run_only_runs_when_asked(self):
        self.assertEqual(check.selected_groups(None, False), {"repository", "bot", "capture"})
        self.assertEqual(check.selected_groups(None, True), {"repository", "bot", "capture", "release"})
        self.assertEqual(check.selected_groups("bot, capture", False), {"bot", "capture"})
        with self.assertRaises(ValueError):
            check.selected_groups("docker", False)

    def test_every_step_is_named_once_and_needs_only_earlier_steps(self):
        seen = []
        for step in check.plan(set(check.GROUPS)):
            self.assertNotIn(step.name, seen)
            for need in step.needs:
                self.assertIn(need, seen, f"{step.name} needs {need}, which does not run before it")
            seen.append(step.name)

    def test_a_step_whose_prerequisite_failed_is_skipped_rather_than_run(self):
        ran = []

        def broken_build(context):
            ran.append("build")
            raise check.StepFailed("does not build")

        def tests(context):
            ran.append("tests")

        steps = [check.Step("capture", "build", "Build", broken_build),
                 check.Step("capture", "tests", "Tests", tests, needs=("build",))]
        results = quietly(check.execute, steps, FakeContext())

        self.assertEqual(ran, ["build"])
        self.assertEqual([result.status for result in results], [check.FAIL, check.SKIP])
        self.assertIn("build", results[1].detail)

    def test_a_passing_command_does_not_put_its_output_into_the_summary(self):
        class NoisyContext(FakeContext):
            def tool(self, name):
                return name

            def require(self, command, what, cwd=None, env=None):
                return "Restoring projects...\n" * 200

        vet = next(step for step in check.bot_steps() if step.name == "vet")
        results = quietly(check.execute, [vet], NoisyContext())

        self.assertEqual(results[0].status, check.PASS)
        self.assertEqual(results[0].detail, "")

    def test_a_step_that_cannot_run_here_is_skipped_with_its_reason(self):
        def windows_only(context):
            raise check.StepSkipped("the app is a Windows program")

        context = FakeContext()
        results = quietly(check.execute, [check.Step("capture", "restore", "Restore", windows_only)], context)

        self.assertEqual(results[0].status, check.SKIP)
        self.assertEqual(results[0].detail, "the app is a Windows program")
        self.assertEqual(context.passed, set())

    def test_a_dry_run_previews_unreleased_changes_when_there_are_any(self):
        changelog = "# Changelog\n\n## Unreleased\n\n- Something new\n\n## v0.1.1-beta — 2026-09-14\n\nText\n"
        self.assertEqual(check.default_version(changelog), check.LOCAL_VERSION)

    def test_without_unreleased_changes_the_newest_release_is_previewed(self):
        changelog = ("# Changelog\n\n## Unreleased\n\n## v0.1.1-beta — 2026-09-14\n\nText\n\n"
                     "## v0.1.0-beta — 2026-09-14\n\nOlder\n")
        self.assertEqual(check.default_version(changelog), "v0.1.1-beta")

    def test_the_summary_names_every_step_and_its_outcome(self):
        results = [check.Result("bot", "vet", "go vet", check.PASS, 1.2),
                   check.Result("release", "installer", "Installer", check.SKIP, 0.0, "Inno Setup 6 is not installed"),
                   check.Result("capture", "format", "Format", check.FAIL, 3.4, "not formatted")]

        text = check.summary(results, 4.6)

        self.assertIn("1 passed, 1 skipped, 1 failed", text)
        self.assertIn("SKIP  release", text)
        self.assertIn("Inno Setup 6 is not installed", text)
        self.assertIn("FAIL  capture", text)


if __name__ == "__main__":
    unittest.main()
