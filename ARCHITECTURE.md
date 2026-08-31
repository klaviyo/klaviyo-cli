# Architecture

The Klaviyo CLI is a Go binary modeled on the Stripe CLI: it wraps the Klaviyo API, manages credentials for multiple accounts, and exposes a typed command for every API operation plus a raw `api` escape hatch.

## Components

```
cmd/klaviyo            entrypoint (main.go only; all logic lives in internal/)
internal/cli           Cobra command tree; flag parsing, output rendering
internal/config        config.toml read/write (account profiles, default account)
internal/keyring       OS keychain storage for account API keys
internal/api           HTTP client: auth header, revision header, retries
api/openapi            vendored Klaviyo OpenAPI spec (source of generated code)
```

Dependency direction: `cli → {config, keyring, api}`. The leaf packages do not import each other or `cli`.

## Credential and account model

A named **account profile** is the unit of auth. Profile metadata — Klaviyo account ID and organization name — lives in `~/.config/klaviyo/config.toml`, written with 0600 permissions (0700 directory). The private API key itself is stored in the OS keychain (`internal/keyring`, a thin wrapper over `zalando/go-keyring`: macOS Keychain, Windows Credential Manager, Linux Secret Service) under service `klaviyo-cli` with the profile name as the user. This follows the GitHub CLI's model, including its opt-in fallback: when no keychain is available, `auth login` fails unless `--insecure-storage` is passed, which stores the key inline in config.toml instead. Each profile's `key_storage` field records where its key lives (`"keyring"`, or empty for file storage — the pre-keychain format, still honored so old configs keep working). `auth migrate` moves file-stored keys into the keychain; `auth list` hints at it while any remain.

Key resolution precedence (`internal/cli/root.go`):

1. `--api-key` flag
2. `KLAVIYO_API_KEY` env var
3. The selected account's stored key (OS keychain, or config.toml for `--insecure-storage` profiles), where the account is chosen by `--account` flag → `KLAVIYO_ACCOUNT` env var → `default_account` in config.toml

`auth login` verifies a key against `GET /api/accounts/` before storing it, and records the account ID and organization name for display in `auth list`.

## API client

`internal/api.Client` is a thin wrapper over `net/http`:

- Base URL `https://a.klaviyo.com`; paths are passed through verbatim.
- Sends `Authorization: Klaviyo-API-Key ...`, `revision`, `Accept`/`Content-Type: application/vnd.api+json`.
- Pins `DefaultRevision` (currently `2026-07-15`); override per invocation with `--revision`.
- Retries: 429 always (honoring `Retry-After`, capped at 60s, context-aware), 5xx only for GET/HEAD, max 4 attempts with exponential backoff.
- `KLAVIYO_API_URL` overrides the base URL for development and tests only; unsupported for normal use.
- Server-supplied text rendered to a terminal (tables, `--jq` strings, non-JSON bodies) is sanitized against escape-sequence injection; piped output stays byte-faithful.
- Non-2xx responses are returned to the caller (not turned into Go errors) so commands can render the API's JSON:API error body.

## Generated resource commands

Every operation in the vendored spec (345 across 23 tags at revision
2026-07-15) becomes a command, following the Stripe CLI's build-time-codegen
model. The generator (`klaviyo-cli-gen`, maintained in an internal Klaviyo
repository) emits:

- `internal/cli/resources_gen.go` — a data table of `opSpec` entries (group,
  name, method, path, params); a generic executor in `resources.go` turns each
  into a Cobra command at startup.
- `internal/api/revision_gen.go` — `DefaultRevision`, pinned to the spec.

Naming is rule-based: tag → group (`Custom Objects` → `custom-objects`);
canonical CRUD on the group's primary resource collapses to `list`/`get`/
`create`/`update`/`delete`; every other operationId is kebab-cased verbatim
(`get_lists_for_profile` → `profiles get-lists-for-profile`). Path params are
positional args; query params become flags (`page[size]` → `--page-size`);
request bodies use `-d` (inline JSON, `@file`, or stdin); list endpoints get
`--paginate`, which follows `links.next` cursors and merges every page's
`data` array.

Freshness: regeneration is a manual task — Klaviyo engineers run the
generator's `regenerate.sh` when klaviyo/openapi publishes a new spec, then
open a PR here; merging and tagging ships the new commands.
The `api` command plus `--revision` covers endpoints newer than the last
release.

## Design decisions

- **Go + Cobra, no Viper.** Cobra is the standard CLI framework (Stripe CLI, GitHub CLI, kubectl). Viper (its usual config companion) is heavyweight and global-state driven; config needs here are one small TOML file, handled explicitly in `internal/config`.
- **Keys in the OS keychain, file storage as opt-in.** Keys live in the OS keychain (the GitHub CLI's model); `--insecure-storage` opts into config.toml (0600) for keychain-less environments. `KLAVIYO_API_KEY`/`--api-key` bypass stored profiles for CI.
- **`internal/` for everything.** The Go compiler forbids external imports of `internal/...`, so nothing here is public API until deliberately moved to `pkg/`.
- **Static binaries.** `CGO_ENABLED=0` everywhere, so releases are dependency-free single files.
- **Module path is the repo URL.** `github.com/klaviyo/klaviyo-cli` — renaming the repo means rewriting every import.

## Roadmap

1. **Docs site** — `docs/` static site in the style of docs.stripe.com/stripe-cli.
2. **Distribution** — Homebrew tap, .deb/.rpm, Docker image (all wired in `.goreleaser.yaml` once the repo is public).
3. **OAuth login** — browser-based flow as an alternative to pasting private keys.
4. **Headless workflows** — fold in the resources-as-files model from [headless-klaviyo](https://github.com/klaviyo/headless-klaviyo) as `pull`/`push` command groups.
