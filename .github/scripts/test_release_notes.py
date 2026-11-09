import unittest
from unittest.mock import patch
import subprocess

from release_notes import extract_notes, parse_tag, previous_tag, validate_module


class ReleaseMetadataTest(unittest.TestCase):
    def test_tag_policy(self):
        for tag, expected in [("v0.4.0", True), ("v1.0.0", False),
                              ("v1.1.0-rc.1", True), ("v1.0.0+build.1", False)]:
            with self.subTest(tag=tag):
                self.assertEqual(validate_module(tag, "module example.com/job\n"), expected)
        self.assertFalse(validate_module("v2.0.0", "module example.com/job/v2\n"))

    def test_invalid_tags(self):
        for tag in ["vfoo", "v01.2.3", "v1.2", "v1.2.3-01", "v1.2.3-", "sub/v1.2.3"]:
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                parse_tag(tag)
        with self.assertRaises(ValueError):
            validate_module("v2.0.0", "module example.com/job\n")
        with self.assertRaises(ValueError):
            validate_module("v1.0.0", "module example.com/job/v2\n")

    def test_notes(self):
        for heading in ["## [0.4.0]", "## [0.4.0] - 2026-11-09"]:
            with self.subTest(heading=heading):
                self.assertEqual(extract_notes("v0.4.0", heading + "\n\n- Added App.\n\n## [0.3.1]\nOld"), "- Added App.\n")

    def test_invalid_notes(self):
        for text in ["## [Unreleased]\n- Change", "## [0x4x0]\n- Change",
                     "## [0.4.0] - Unreleased\n- Change", "## [0.4.0]\n\n",
                     "## [0.4.0]\n### Added\n[0.4.0]: https://example.com",
                     "## [0.4.0]\n- One\n## [0.4.0]\n- Two",
                     "## [0.4.0] - 2026-02-30\n- Change"]:
            with self.subTest(text=text), self.assertRaises(ValueError):
                extract_notes("v0.4.0", text)

    @patch("release_notes.subprocess.run")
    def test_first_commit(self, run):
        run.return_value = subprocess.CompletedProcess([], 1, "", "")
        self.assertIsNone(previous_tag("v0.1.0", "module example.com/job"))

    @patch("release_notes.subprocess.check_output")
    @patch("release_notes.subprocess.run")
    def test_previous_tag_filters(self, run, output):
        run.return_value = subprocess.CompletedProcess([], 0, "abc\n", "")
        output.return_value = "v2.0.0\nsub/v1.0.0\nvbad\nv0.3.1\n"
        self.assertEqual(previous_tag("v0.4.0", "module example.com/job"), "v0.3.1")
        output.return_value = ""
        self.assertIsNone(previous_tag("v0.1.0", "module example.com/job"))


if __name__ == "__main__":
    unittest.main()
