package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fakeBun(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake runtime; release compilation covers Windows")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bun"), []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
}

func TestMissingRuntime(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := Launch(nil); err == nil || !strings.Contains(err.Error(), "klaviyo tui setup") {
		t.Fatalf("missing setup advice: %v", err)
	}
}

func TestOldRuntime(t *testing.T) {
	fakeBun(t, "echo 1.3.13\n")
	if err := Setup(); err == nil || !strings.Contains(err.Error(), "upgrade Bun") {
		t.Fatalf("old runtime accepted: %v", err)
	}
}

func TestLaunchNeverInstalls(t *testing.T) {
	fakeBun(t, "case \"$1\" in --version) echo 1.3.14;; *) exit 99;; esac\n")
	if err := Launch(nil); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("unexpected launch: %v", err)
	}
}

func TestSetupExtractsEmbeddedAssetsAndPinsInstall(t *testing.T) {
	fakeBun(t, `case "$1" in
--version) echo 1.3.14;;
install) test "$2" = --frozen-lockfile && test "$3" = --ignore-scripts;;
-e) test -f src/main.tsx && test -f package.json && test -f bun.lock;;
*) exit 99;;
esac
`)
	if err := Setup(); err != nil {
		t.Fatal(err)
	}
	dir, err := runtimeDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".ready")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "main.tsx")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "test")); !os.IsNotExist(err) {
		t.Fatal("tests should not ship in binary")
	}
}

func TestFailedSetupDoesNotMarkReady(t *testing.T) {
	fakeBun(t, "case \"$1\" in --version) echo 1.3.14;; *) exit 1;; esac\n")
	if err := Setup(); err == nil {
		t.Fatal("failed install succeeded")
	}
	dir, err := runtimeDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".ready")); !os.IsNotExist(err) {
		t.Fatal("failed install marked ready")
	}
}
