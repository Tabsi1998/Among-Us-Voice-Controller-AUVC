import sys
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))
import verify_bootstrap  # noqa: E402


class VerifyBootstrapTests(unittest.TestCase):
    """The script ran in no check and rotted (#194). Running it here keeps it honest."""

    def test_every_required_file_exists_today(self):
        missing = [path for path in verify_bootstrap.REQUIRED if not (verify_bootstrap.ROOT / path).is_file()]

        self.assertEqual(missing, [])

    def test_the_files_deleted_in_september_are_not_required(self):
        for path in ("deploy/README.md", "docs/requirements.md", "docs/roadmap.md",
                     "docs/bootstrap-validation.md"):
            self.assertNotIn(path, verify_bootstrap.REQUIRED)

    def test_a_missing_scaffold_file_is_reported(self):
        with mock.patch.object(verify_bootstrap.verify_repository, "check", return_value=[]), \
                mock.patch.object(verify_bootstrap, "REQUIRED", ("docs/not-there.md",)):
            failures = verify_bootstrap.check()

        self.assertIn("Missing required file: docs/not-there.md", failures)

    def test_the_import_check_is_the_one_of_verify_repository(self):
        with mock.patch.object(verify_bootstrap.verify_repository, "check", return_value=["bot: changed"]):
            failures = verify_bootstrap.check()

        self.assertEqual(failures[0], "bot: changed")


if __name__ == "__main__":
    unittest.main()
