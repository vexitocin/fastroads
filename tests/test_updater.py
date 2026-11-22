import json
import tempfile
import unittest
import zipfile
from pathlib import Path

from updater import _extract_release_zip, parse_version


class UpdaterTests(unittest.TestCase):
    def test_release_versions_compare_numerically(self):
        self.assertLess(parse_version("v2.0.9"), parse_version("v2.0.10"))

    def test_rejects_non_release_versions(self):
        with self.assertRaises(ValueError):
            parse_version("latest")

    def test_validates_and_extracts_release_archive(self):
        with tempfile.TemporaryDirectory() as temporary_directory:
            temporary = Path(temporary_directory)
            archive_path = temporary / "release.zip"
            destination = temporary / "package"
            destination.mkdir()
            with zipfile.ZipFile(archive_path, "w") as archive:
                archive.writestr(
                    "fastroads-update.json",
                    json.dumps({"version": "v2.0.5", "channel": "normal"}),
                )
                archive.writestr("fastroads.exe", "app")
                archive.writestr("_internal/runtime.dat", "runtime")

            _extract_release_zip(archive_path, destination, "v2.0.5", "normal")

            self.assertEqual((destination / "fastroads.exe").read_text(), "app")
            self.assertTrue((destination / "_internal/runtime.dat").is_file())

    def test_rejects_archive_path_traversal(self):
        with tempfile.TemporaryDirectory() as temporary_directory:
            temporary = Path(temporary_directory)
            archive_path = temporary / "release.zip"
            destination = temporary / "package"
            destination.mkdir()
            with zipfile.ZipFile(archive_path, "w") as archive:
                archive.writestr("../outside.txt", "not allowed")

            with self.assertRaises(ValueError):
                _extract_release_zip(archive_path, destination, "v2.0.5", "normal")

            self.assertFalse((temporary / "outside.txt").exists())


if __name__ == "__main__":
    unittest.main()
