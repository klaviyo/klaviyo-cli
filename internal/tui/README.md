# Optional command browser

The Go binary embeds `assets/src`, `package.json`, `bun.lock`, and `tsconfig.json`.
Neither building nor running ordinary CLI commands requires JavaScript. `klaviyo
tui setup` checks for Bun >= 1.3.14, extracts these files under the OS user cache
directory (`klaviyo-tui/<asset digest>`), and runs a frozen, production-only install
with lifecycle scripts disabled. It verifies native OpenTUI imports before marking
that version ready. Upgrading UI assets requires running setup again. Launch never
installs dependencies or downloads a runtime.

OpenTUI 0.8.1 supports Bun >= 1.3.14 or Node >= 26.4.0. This implementation uses
Bun for TSX execution, native FFI, and process handling; the Go project has no
existing JavaScript runtime to reuse. This is an explicit opt-in tradeoff. There is
no separate TUI repository or release to install.

## Command discovery and execution

`klaviyo tui catalog` is an internal, hidden JSON endpoint. It walks the live Cobra
tree, including generated stable/beta resources, local/inherited flags, shorthand
names, required flags, repeatability, descriptions, and argument syntax from `Use`.
It does not invoke command handlers, validation, authentication, or pre-run hooks.
The browser rebuilds its in-memory index at launch; no stale on-disk catalog exists.
The management, completion, and help commands are excluded from browsing.

`--tui` is reserved before `--`. The launcher removes it without parsing or executing
the command. Groups scope the browser; a complete command opens an editable form.
Untouched forms retain the exact launch tokens (including aliases, empty arguments,
and ordering). Editing rebuilds argv from fields, repeatable flags, extra options,
and a separate literal-argument field. No shell evaluates these arguments. Cobra
remains the authority for validation. Flags with unusual/custom shapes can be
entered in extra options. Global options on a scoped launch prefill subsequent forms.

The launching binary's absolute path is passed to Bun for both catalog reads and
command execution. The caller's working directory and environment are retained,
including relative `@file` payloads. On Enter, the renderer is destroyed and its
`closed` promise awaited; the child inherits the original terminal. Both waiting
parents survive terminal SIGINT so Ctrl+C can interrupt a child. After exit, Enter
returns to the browser and `q` quits.

Favorites, up to 100 recent commands, last-used field values, and theme are stored
in `${XDG_CONFIG_HOME:-~/.config}/klaviyo-tui/state.json`. On Unix the app directory
is restricted to 0700 and state to 0600; writes reject symlink/hardlink targets.
`KLAVIYO_TUI_STATE` overrides the file without changing its parent permissions.
History can contain argument values: prefer `KLAVIYO_API_KEY` over a literal
`--api-key`. Delete the state file to clear history. POSIX permissions and terminal
handoff are tested on macOS; Windows persistence/terminal behavior needs validation.

## Development and checks

```sh
make build
cd internal/tui/assets
bun install --frozen-lockfile --ignore-scripts
bun test
bun run typecheck
cd ../../..
make test
make lint
bin/klaviyo tui setup
bin/klaviyo --tui version
```

The final command must show a form without executing. Press Enter to run the
version command, Enter to return, and Escape to exit. Also try:

```sh
bin/klaviyo --tui
bin/klaviyo profiles --tui
bin/klaviyo --account demo profiles get placeholder --help --tui
```

The help example is safe without credentials and must not send an API request.
Use arrows/Tab to navigate, type to fuzzy-search names/descriptions, Ctrl+F to
favorite, Ctrl+Y to copy, Shift+arrows to scroll details, and `?` for keyboard help.
Clipboard uses `pbcopy` on macOS, falling back to terminal OSC 52 support.

Go tests cover discovery, collisions, setup failures, pinned installs and launch
without dependencies. Bun tests cover index/search, launch/forms, argument
boundaries and private state. The optional PTY smoke script runs only help/version
and a disposable, interruptible version fixture to verify both waiting parents
survive child Ctrl+C:

```sh
python3 internal/tui/smoke.py ./bin/klaviyo
```
