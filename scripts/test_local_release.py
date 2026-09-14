"""Tests for scripts/local_release.py.

Run with:
    python -m unittest discover -s scripts -p "test_*.py"
"""

import re
import tempfile
import unittest
from dataclasses import replace
from pathlib import Path

import local_release as release

READY = release.State(
    clean=True, head="abc123", head_on_main=True, notes_found=True, tag_commit=None,
    release_exists=False, gh_logged_in=True, inno_setup=True,
)


class VersionTests(unittest.TestCase):
    def test_pre_releases_before_and_after_one_point_zero_are_built_here(self):
        for tag in ("v0.1.2-beta", "v1.2.3-beta", "v1.2.3-alpha", "v1.0.0-rc.1", "v10.20.30-beta.2"):
            release.check_version(tag)

    def test_a_release_without_a_suffix_goes_to_github(self):
        for tag in ("v1.0.0", "v1.2.3"):
            with self.assertRaisesRegex(release.Refused, "built on GitHub"):
                release.check_version(tag)

    def test_anything_else_is_refused(self):
        for tag in ("1.2.3-beta", "v1.2-beta", "v1.2.3-dev", "v1.2.3-beta-final", "v1.2.3 -beta", ""):
            with self.assertRaises(release.Refused, msg=tag):
                release.check_version(tag)

    def test_the_release_workflow_leaves_pre_release_tags_alone(self):
        """GitHub's own build must not publish what this script publishes."""
        workflow = (release.ROOT / ".github" / "workflows" / "release.yml").read_text(encoding="utf-8")
        tags = re.search(r"tags:\s*\[([^\]]*)\]", workflow)

        self.assertIsNotNone(tags, "release.yml has no tag filter")
        patterns = [pattern.strip().strip("'\"") for pattern in tags.group(1).split(",")]
        self.assertEqual(patterns, ["v*", "!v*-*"])


class RefusalTests(unittest.TestCase):
    def test_a_ready_repository_is_not_refused(self):
        self.assertEqual(release.refusals("v1.2.3-beta", READY, publish=True), [])

    def test_every_unsafe_state_is_refused_with_a_reason(self):
        cases = {
            "uncommitted changes": replace(READY, clean=False),
            "not on origin/main": replace(READY, head_on_main=False),
            "no section for": replace(READY, notes_found=False),
            "already exists; releases are never replaced": replace(READY, release_exists=True),
            "already exists on another commit": replace(READY, tag_commit="def456"),
            "gh auth login": replace(READY, gh_logged_in=False),
            "Inno Setup 6": replace(READY, inno_setup=False),
        }
        for expected, state in cases.items():
            reasons = release.refusals("v1.2.3-beta", state, publish=True)
            self.assertEqual(len(reasons), 1, expected)
            self.assertIn(expected, reasons[0])

    def test_a_dry_run_needs_neither_a_login_nor_inno_setup(self):
        state = replace(READY, gh_logged_in=False, inno_setup=False)
        self.assertEqual(release.refusals("v1.2.3-beta", state, publish=False), [])

    def test_a_tag_pushed_before_a_failed_upload_can_be_finished(self):
        state = replace(READY, tag_commit=READY.head)
        self.assertEqual(release.refusals("v1.2.3-beta", state, publish=True), [])


class ResultTests(unittest.TestCase):
    def report(self, *results):
        return {"results": [dict(group=group, name=name, describe=name, status=status)
                            for group, name, status in results]}

    def test_a_failed_check_stops_the_release(self):
        report = self.report(("bot", "race", "failed"), ("release", "installer", "passed"))
        self.assertEqual(release.failed_checks(report, publish=True), ["bot: race"])

    def test_publishing_needs_the_installer_to_have_been_built(self):
        report = self.report(("bot", "race", "passed"), ("release", "installer", "skipped"))

        self.assertEqual(release.failed_checks(report, publish=False), [])
        self.assertEqual(release.failed_checks(report, publish=True), ["release: the installer was not built"])

    def test_missing_files_are_named(self):
        with tempfile.TemporaryDirectory() as folder:
            (Path(folder) / "AmongUsVoiceCapture-win-x64.zip").write_bytes(b"PK")
            self.assertEqual(release.missing_assets(Path(folder)),
                             ["AmongUsVoiceCapture-Setup-win-x64.exe", "SHA256SUMS"])

    def test_checksums_have_to_list_the_installer_and_the_zip(self):
        complete = ("a" * 64 + "  AmongUsVoiceCapture-Setup-win-x64.exe\n"
                    + "b" * 64 + "  AmongUsVoiceCapture-win-x64.zip\n")
        self.assertEqual(release.unlisted_in_checksums(complete), [])
        self.assertEqual(release.unlisted_in_checksums("b" * 64 + "  AmongUsVoiceCapture-win-x64.zip\n"),
                         ["AmongUsVoiceCapture-Setup-win-x64.exe"])


if __name__ == "__main__":
    unittest.main()
