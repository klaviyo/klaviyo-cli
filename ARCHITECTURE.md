# Architecture

The Klaviyo CLI is a Go binary modeled on the Stripe CLI: it wraps the Klaviyo API, manages credentials for multiple accounts, and exposes both a raw `api` escape hatch and (eventually) typed resource commands.

## Components

```
cmd/klaviyo            entrypoint (main.go only; all logic lives in internal/)
internal/cli           Cobra command tree; flag parsing, output rendering
internal/config        config.toml read/write (account profiles, default account)
internal/api           HTTP client: auth header, revision header, retries
```

Dependency direction: `cli → {config, api}`. The leaf packages do not import each other or `cli`.

## Credential and account model

A named **account profile** is the unit of auth. Profiles — Klaviyo account ID, organization name, and the private API key — live in `~/.config/klaviyo/config.toml`, written with 0600 permissions (0700 directory). This matches the Stripe CLI's model; migrating keys to the OS keychain (as the GitHub CLI does) is planned in [issue #1](https://github.com/klaviyo/klaviyo-cli/issues/1).

Key resolution precedence (`internal/cli/root.go`):

1. `--api-key` flag
2. `KLAVIYO_API_KEY` env var
3. Keychain entry for the selected account, where the account is chosen by `--account` flag → `KLAVIYO_ACCOUNT` env var → `default_account` in config.toml

`auth login` verifies a key against `GET /api/accounts/` before storing it, and records the account ID and organization name for display in `auth list`.

## API client

`internal/api.Client` is a thin wrapper over `net/http`:

- Base URL `https://a.klaviyo.com`; paths are passed through verbatim.
- Sends `Authorization: Klaviyo-API-Key ...`, `revision`, `Accept`/`Content-Type: application/vnd.api+json`.
- Pins `DefaultRevision` (currently `2026-07-15`); override per invocation with `--revision`.
- Retries: 429 always (honoring `Retry-After`), 5xx only for GET/HEAD, max 4 attempts with exponential backoff.
- Non-2xx responses are returned to the caller (not turned into Go errors) so commands can render the API's JSON:API error body.

## Design decisions

- **Go + Cobra, no Viper.** Cobra is the standard CLI framework (Stripe CLI, GitHub CLI, kubectl). Viper (its usual config companion) is heavyweight and global-state driven; config needs here are one small TOML file, handled explicitly in `internal/config`.
- **Keys in the config file, keychain later.** v0 stores keys in config.toml (0600) like the Stripe CLI, keeping storage simple and headless-friendly. OS keychain storage — the GitHub CLI's current default — is tracked in issue #1. `KLAVIYO_API_KEY`/`--api-key` bypass stored profiles for CI.
- **`internal/` for everything.** The Go compiler forbids external imports of `internal/...`, so nothing here is public API until deliberately moved to `pkg/`.
- **Static binaries.** `CGO_ENABLED=0` everywhere, so releases are dependency-free single files.
- **Module path is the repo URL.** `github.com/klaviyo/klaviyo-cli` — renaming the repo means rewriting every import.

## Roadmap

1. **Typed resource commands** — `klaviyo profiles list`, `klaviyo campaigns get <id>`, etc., generated or hand-mapped from the [Klaviyo OpenAPI spec](https://github.com/klaviyo/openapi); shared pagination (`--all`), `--output json|table`.
2. **Docs site** — `docs/` static site in the style of docs.stripe.com/stripe-cli.
3. **Distribution** — Homebrew tap, .deb/.rpm, Docker image (all wired in `.goreleaser.yaml` once the repo is public).
4. **OS keychain key storage** — [issue #1](https://github.com/klaviyo/klaviyo-cli/issues/1).
5. **OAuth login** — browser-based flow as an alternative to pasting private keys.
6. **Headless workflows** — fold in the resources-as-files model from [headless-klaviyo](https://github.com/klaviyo/headless-klaviyo) as `pull`/`push` command groups.
