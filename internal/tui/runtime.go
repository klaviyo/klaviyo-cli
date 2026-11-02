// Package tui ships the optional OpenTUI browser inside the CLI binary.
package tui

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
	"golang.org/x/term"
)

//go:embed assets/package.json assets/bun.lock assets/tsconfig.json assets/src
var assets embed.FS

const minimumBun = "v1.3.14"

func runtimeDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	err = fs.WalkDir(assets, "assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := assets.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%s\x00", path, data)
		return nil
	})
	return filepath.Join(base, "klaviyo-tui", fmt.Sprintf("%x", h.Sum(nil)[:12])), err
}

func bunRuntime() (string, error) {
	bin, err := exec.LookPath("bun")
	if err != nil {
		return "", fmt.Errorf("OpenTUI requires Bun >= %s; install Bun from https://bun.sh, then run `klaviyo tui setup`", minimumBun[1:])
	}
	out, err := exec.Command(bin, "--version").Output()
	v := "v" + strings.TrimSpace(string(out))
	if err != nil || !semver.IsValid(v) || semver.Compare(v, minimumBun) < 0 {
		return "", fmt.Errorf("OpenTUI requires Bun >= %s; upgrade Bun, then run `klaviyo tui setup`", minimumBun[1:])
	}
	return bin, nil
}

func verifyRuntime(bin, dir string) error {
	cmd := exec.Command(bin, "-e", `await import("@opentui/core"); await import("@opentui/react")`)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("OpenTUI runtime check failed: %w\n%s", err, out)
	}
	return nil
}

// Setup is the only path that downloads dependencies. The digest isolates releases.
func Setup() error {
	bin, err := bunRuntime()
	if err != nil {
		return err
	}
	dir, err := runtimeDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, ".ready")); err != nil && !os.IsNotExist(err) {
		return err
	}
	err = fs.WalkDir(assets, "assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(dir, strings.TrimPrefix(path, "assets"))
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o700)
		}
		data, err := assets.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o600)
	})
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, "install", "--frozen-lockfile", "--ignore-scripts", "--production")
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installing OpenTUI: %w", err)
	}
	if err := verifyRuntime(bin, dir); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, ".ready"), []byte("ready\n"), 0o600); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "OpenTUI is ready. Run `klaviyo --tui`.")
	return nil
}

// Launch preserves the calling directory and executes the same CLI binary.
func Launch(args []string) error {
	bin, err := bunRuntime()
	if err != nil {
		return err
	}
	dir, err := runtimeDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, ".ready")); err != nil {
		return fmt.Errorf("optional TUI dependencies are not installed for this build; run `klaviyo tui setup`")
	}
	if err := verifyRuntime(bin, dir); err != nil {
		return fmt.Errorf("%w; run `klaviyo tui setup`", err)
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("klaviyo --tui needs an interactive terminal")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, append([]string{filepath.Join(dir, "src", "main.tsx")}, args...)...)
	cmd.Env = append(os.Environ(), "KLAVIYO_TUI_BIN="+executable)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// SIGINT reaches the whole foreground group; keep this waiting launcher alive.
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)
	return cmd.Run()
}
