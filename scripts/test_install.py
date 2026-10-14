"""Runs install.sh against release files served from a local directory."""

import functools
import hashlib
import http.server
import os
from pathlib import Path
import platform
import shutil
import subprocess
import tempfile
import threading
import unittest

SCRIPT = Path(__file__).resolve().parent / "install.sh"
MODULE = "github.com/tonyredondo/dd-ci-testing-poc"
OS = {"Linux": "linux", "Darwin": "darwin"}.get(platform.system())
ARCH = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(platform.machine().lower())
FAKE_DDTO = "#!/bin/sh\necho usage: ddto\n"


class Quiet(http.server.SimpleHTTPRequestHandler):
    def log_message(self, *args):
        pass


@unittest.skipUnless(OS and ARCH and shutil.which("bash"), "install.sh targets Unix shells on amd64 and arm64")
class InstallTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.assets = self.root / "assets"
        self.assets.mkdir()
        self.dir = self.root / "bin"
        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), functools.partial(Quiet, directory=self.assets))
        threading.Thread(target=server.serve_forever, daemon=True).start()
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        self.url = f"http://127.0.0.1:{server.server_port}"
        self.asset = f"ddto-{OS}-{ARCH}"

    def release(self, data=FAKE_DDTO, checksum=None):
        (self.assets / self.asset).write_text(data)
        digest = checksum or hashlib.sha256(data.encode()).hexdigest()
        (self.assets / "SHA256SUMS").write_text(f"{digest}  {self.asset}\n{'0' * 64}  ddto-other-arch\n")

    def install(self, *args, path=None, github_path=None):
        env = {key: value for key, value in os.environ.items() if not key.startswith(("DDTO_", "GITHUB_"))}
        env.update(DDTO_DOWNLOAD_URL=self.url, DDTO_INSTALL_DIR=str(self.dir), PATH=path or os.environ["PATH"])
        if github_path:
            env["GITHUB_PATH"] = str(github_path)
        return subprocess.run(["bash", str(SCRIPT), *args], env=env, capture_output=True, text=True, timeout=60)

    def fake_go(self):
        """Returns a PATH whose go records its arguments and installs a fake ddto in GOBIN."""
        tools = self.root / "tools"
        tools.mkdir()
        go = tools / "go"
        go.write_text(f'#!/bin/sh\necho "$@" > "{self.root}/go-args"\nprintf "%s" "{FAKE_DDTO}" > "$GOBIN/ddto"\n'
                      'chmod +x "$GOBIN/ddto"\n')
        go.chmod(0o755)
        return f"{tools}{os.pathsep}{os.environ['PATH']}"

    def test_installs_the_checked_release_executable(self):
        self.release()
        result = self.install()
        self.assertEqual(result.returncode, 0, result.stderr)
        installed = self.dir / "ddto"
        self.assertEqual(installed.read_text(), FAKE_DDTO)
        self.assertTrue(os.access(installed, os.X_OK))
        self.assertIn("(release latest)", result.stderr)

    def test_rejects_a_checksum_mismatch_without_falling_back(self):
        self.release(checksum="f" * 64)
        result = self.install(path=self.fake_go())
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("does not match SHA256SUMS", result.stderr)
        self.assertFalse((self.dir / "ddto").exists())
        self.assertFalse((self.root / "go-args").exists())

    def test_builds_with_go_install_without_a_release_executable(self):
        result = self.install("--version", "v1.2.3", path=self.fake_go())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(f"no release executable {self.asset}", result.stderr)
        self.assertEqual((self.root / "go-args").read_text().split(), ["install", f"{MODULE}/cmd/ddto@v1.2.3"])
        self.assertIn("(go install @v1.2.3)", result.stderr)

    def test_adds_the_directory_to_github_path(self):
        self.release()
        github_path = self.root / "github_path"
        github_path.write_text("")
        result = self.install(github_path=github_path)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(github_path.read_text().split(), [str(self.dir.resolve())])


if __name__ == "__main__":
    unittest.main()
