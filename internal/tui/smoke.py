#!/usr/bin/env python3
"""Opt-in Unix PTY smoke: built CLI + completed tui setup; no API calls."""
import errno
import fcntl
import os
from pathlib import Path
import pty
import select
import shutil
import signal
import struct
import sys
import tempfile
import termios
import time


def scenario(binary, args, execute=False, interrupt=False):
    with tempfile.TemporaryDirectory(prefix="klaviyo-tui-smoke-") as directory:
        if interrupt:
            # Replace only this disposable executable after metadata was loaded,
            # giving the real Go/Bun parents a controllable, harmless child.
            binary = shutil.copy2(binary, str(Path(directory) / "klaviyo"))
        pid, fd = pty.fork()
        if pid == 0:
            os.chdir(directory)
            os.environ["TERM"] = "xterm-256color"
            os.environ["KLAVIYO_TUI_STATE"] = str(Path(directory) / "state.json")
            os.environ["KLAVIYO_NO_UPDATE_NOTIFIER"] = "1"
            os.execv(binary, [binary, *args])
        fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 130, 0, 0))
        output = bytearray()
        reaped = False

        def wait_for(text, timeout=15):
            end = time.monotonic() + timeout
            while time.monotonic() < end:
                if text.encode() in output:
                    return
                if select.select([fd], [], [], 0.1)[0]:
                    try:
                        chunk = os.read(fd, 65536)
                    except OSError as error:
                        if error.errno == errno.EIO:
                            break
                        raise
                    output.extend(chunk)
                    # Respond to terminal capability queries used by OpenTUI.
                    if b"\x1b[6n" in chunk:
                        os.write(fd, b"\x1b[1;1R")
            raise AssertionError(f"Missing {text!r}: {bytes(output)[-4000:]!r}")

        try:
            wait_for("ready" if execute else "groups")
            assert b"exited 0" not in output, "command executed before Enter"
            if execute:
                if interrupt:
                    fixture = Path(directory) / "child"
                    fixture.write_text(f"#!{sys.executable}\nimport os, signal, sys, time\nassert sys.argv[1:] == ['version']\nsignal.signal(signal.SIGINT, lambda *_: sys.exit(130))\nprint('interrupt-fixture-ready', os.getpgrp(), os.tcgetpgrp(0), flush=True)\ntime.sleep(30)\n")
                    fixture.chmod(0o700)
                    os.replace(fixture, binary)
                output.clear()
                os.write(fd, b"\r")
                if interrupt:
                    wait_for("interrupt-fixture-ready")
                    os.write(fd, b"\x03")
                    wait_for("exited 130")
                else:
                    wait_for("exited 0")
                    assert b"klaviyo dev (commit" in output or b"Usage:" in output
                output.clear()
                os.write(fd, b"\r")
                wait_for("groups")
            # Escape once per group then once at root.
            for _ in range(4):
                os.write(fd, b"\x1b")
                time.sleep(0.15)
            end = time.monotonic() + 5
            while time.monotonic() < end:
                if select.select([fd], [], [], 0.1)[0]:
                    try:
                        output.extend(os.read(fd, 65536))
                    except OSError as error:
                        if error.errno != errno.EIO:
                            raise
                done, status = os.waitpid(pid, os.WNOHANG)
                if done:
                    reaped = True
                    assert os.waitstatus_to_exitcode(status) == 0, status
                    break
                time.sleep(0.1)
            assert reaped, f"browser did not exit cleanly: {bytes(output)[-2000:]!r}"
            print("PASS", " ".join(args), "(SIGINT fixture)" if interrupt else "")
        finally:
            if not reaped:
                os.killpg(pid, signal.SIGKILL)
                os.waitpid(pid, 0)
            os.close(fd)


if __name__ == "__main__":
    executable = str(Path(sys.argv[1]).resolve())
    scenario(executable, ["--tui"])
    scenario(executable, ["profiles", "--tui"])
    scenario(executable, ["version", "--tui"], execute=True)
    scenario(executable, ["--account", "demo", "profiles", "get", "placeholder", "--help", "--tui"], execute=True)
    scenario(executable, ["version", "--tui"], execute=True, interrupt=True)
