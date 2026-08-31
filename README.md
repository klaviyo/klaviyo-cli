# Klaviyo CLI

The Klaviyo CLI lets you build, test, and manage your [Klaviyo](https://www.klaviyo.com) integration from the terminal — in the spirit of the Stripe CLI.

With the CLI, you can:

- Call every Klaviyo API operation as a typed command (345 commands across 23 groups, generated from the [OpenAPI spec](https://github.com/klaviyo/openapi))
- Authenticate once per account and switch between accounts
- Script against the API with built-in jq filtering and cursor pagination
- Hit any endpoint raw with `klaviyo api`, including ones newer than your CLI build

> Status: early development, private preview.

## Installation

The repo is private, so both channels need GitHub credentials with access to `klaviyo/klaviyo-cli`.

**Binary releases** (macOS, Linux, and Windows; amd64 and arm64) are on the [Releases page](https://github.com/klaviyo/klaviyo-cli/releases/latest). With the [GitHub CLI](https://cli.github.com):

```bash
gh release download -R klaviyo/klaviyo-cli --pattern '*darwin_arm64*'
tar -xzf klaviyo-cli_*_darwin_arm64.tar.gz klaviyo
mv klaviyo /usr/local/bin/   # or anywhere on your PATH
```

**From source** (Go 1.25+, with git authenticated to GitHub — for example via `gh auth setup-git`):

```bash
GOPRIVATE=github.com/klaviyo/* go install github.com/klaviyo/klaviyo-cli/cmd/klaviyo@latest
```

Package managers (Homebrew tap, .deb/.rpm, Docker image) are planned for when the repo goes public.

### Upgrading

The CLI checks GitHub for a newer release at most once per day and prints a notice to stderr — only on interactive terminals, never in CI or when piped. It never self-updates: upgrade through the channel you installed with. Opt out of the check with `KLAVIYO_NO_UPDATE_NOTIFIER=1`.

## Quickstart

```bash
# Store a private API key for an account (verified before saving)
klaviyo auth login

# Confirm credentials
klaviyo auth status

# Typed commands for every API operation
klaviyo metrics list
klaviyo profiles list --filter 'equals(email,"someone@example.com")'
klaviyo profiles get 01ABC123 --fields-profile email,first_name
klaviyo lists list --paginate          # follow cursors, merge all pages
klaviyo events create -d @event.json

# Filter any response with the built-in jq (no jq install needed)
klaviyo lists list --jq '.data[].attributes.name'
klaviyo profiles list --paginate --jq '.data | length'

# Or call any endpoint raw
klaviyo api /api/metrics/
klaviyo api POST /api/events/ -d @event.json
```

## Commands

Core commands:

| Command | Description |
| --- | --- |
| `klaviyo auth login` | Store an API key for a named account (verified first) |
| `klaviyo auth logout <account>` | Remove an account and its key |
| `klaviyo auth list` | List configured accounts |
| `klaviyo auth switch <account>` | Set the default account |
| `klaviyo auth status` | Verify credentials for the selected account |
| `klaviyo auth migrate` | Move file-stored API keys into the OS keychain |
| `klaviyo api [method] <path>` | Raw authenticated API request (defaults to GET) |
| `klaviyo config` | Show or edit CLI configuration (`--list`, `--set`, `-e`) |
| `klaviyo open <shortcut>` | Open Klaviyo dashboard or docs pages in your browser |
| `klaviyo completion <shell>` | Generate shell completion scripts |
| `klaviyo version` | Print the CLI version |

Resource commands cover every operation in the Klaviyo API — one command per operation, in groups:

```
accounts campaigns catalogs client conversations coupons custom-objects
data-privacy events flows forms images lists metrics profiles reporting
reviews segments tags templates tracking-settings web-feeds webhooks
```

Conventions across all resource commands:

- Canonical CRUD is `list`, `get`, `create`, `update`, `delete`; everything else keeps its operation name (`klaviyo profiles get-lists-for-profile`).
- Path parameters are positional arguments; query parameters are flags (`page[size]` becomes `--page-size`).
- Request bodies use `-d` / `--data`: inline JSON, `@file`, or `-` for stdin.
- List commands accept `--paginate` to follow cursors and merge every page's `data` array.

Run `klaviyo <group> --help` for a group's commands, or `klaviyo <group> <command> --help` for its flags. The full reference is the CLI's own help output.

## Authentication and accounts

`klaviyo auth login` verifies a [private API key](https://developers.klaviyo.com/en/docs/authenticate_) against the API, then stores it as a named account profile. The first account added becomes the default. Store as many accounts as you like:

```bash
klaviyo auth login --account prod
klaviyo auth login --account staging
klaviyo auth list                      # * marks the default
klaviyo auth switch staging            # change the default

# One-off override for a single command:
klaviyo api /api/metrics/ --account prod
```

The key used for a request resolves in this order:

1. `--api-key` flag
2. `KLAVIYO_API_KEY` environment variable
3. The selected account's stored key, where the account is chosen by `--account` flag, then `KLAVIYO_ACCOUNT`, then the configured default

Profiles live in `~/.config/klaviyo/config.toml`; the API keys themselves are stored in the OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service). Where no keychain is available — headless Linux, containers — pass `--insecure-storage` to `auth login` to store the key in the config file instead, written with `0600` permissions. Keys stored in the file by older CLI versions keep working; run `klaviyo auth migrate` to move them into the keychain. In CI, skip stored accounts entirely and set `KLAVIYO_API_KEY`.

## Output and scripting

- **Terminals get tables, pipes get JSON.** List responses render as aligned tables on an interactive terminal; piped or redirected output is always pretty-printed JSON, so scripts never parse table text.
- **`--jq <expr>`** filters any response through a built-in jq interpreter ([gojq](https://github.com/itchyny/gojq)) — no jq install needed. Following jq convention, string results print raw and other values print as JSON, one result per line.
- **`--paginate`** follows `links.next` cursors and merges all pages' `data` into one response (GET list endpoints only). Combines with `--jq`, which runs on the merged result.
- **`--revision <date>`** overrides the pinned API revision header for a single call.
- **Errors:** non-2xx responses print the API's JSON:API error body and exit non-zero.

## Shell completion

Completion covers every command, flag, and configured account name:

```bash
# zsh (bash/fish/powershell also supported)
klaviyo completion zsh > "${fpath[1]}/_klaviyo"
```

## Environment variables

| Variable | Purpose |
| --- | --- |
| `KLAVIYO_API_KEY` | API key for requests, bypassing stored accounts (below `--api-key` in precedence) |
| `KLAVIYO_ACCOUNT` | Named account to use (below `--account` in precedence) |
| `KLAVIYO_CONFIG_DIR` | Config directory override (default `~/.config/klaviyo`) |
| `KLAVIYO_NO_UPDATE_NOTIFIER` | Disable the update check (also disabled when `CI` is set) |
| `VISUAL`, `EDITOR` | Editor for `klaviyo config -e` |
| `KLAVIYO_API_URL` | Base URL override for development and tests only; unsupported for normal use |

## Development

Requires Go 1.25+; everything else self-installs. See [ARCHITECTURE.md](ARCHITECTURE.md) for how the pieces fit together.

```bash
make build      # builds bin/klaviyo
make test       # go test -race ./...
make lint       # installs the pinned golangci-lint into ./bin, then runs it
make fmt        # gofumpt + goimports via golangci-lint fmt
make generate   # regenerate resource commands from the vendored OpenAPI spec
```

CI runs lint, tests on all three platforms, a snapshot release build, and fails if `go generate` or `go mod tidy` would change the committed tree. A scheduled workflow pulls the latest OpenAPI spec, regenerates, and opens a PR when anything changed.

Releases are cut by pushing a `v*` tag; GoReleaser builds and publishes the binaries.

Found a bug or want a feature? [Open an issue](https://github.com/klaviyo/klaviyo-cli/issues).

## License

[MIT](LICENSE)
