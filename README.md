# Klaviyo CLI

Manage Klaviyo from the command line. The CLI wraps the [Klaviyo API](https://developers.klaviyo.com/en/reference/api_overview), handles authentication for one or more accounts, and pretty-prints responses — in the spirit of the Stripe CLI.

> Status: early development, private preview.

## Install

Binary releases are built for macOS, Linux, and Windows (see the repo's Releases page). From source:

```bash
go install github.com/klaviyo/klaviyo-cli/cmd/klaviyo@latest
```

## Quickstart

```bash
# Store a private API key for an account (verified before saving)
klaviyo auth login

# Confirm credentials
klaviyo auth status

# Typed commands for every API operation (345 across 23 groups,
# generated from the OpenAPI spec)
klaviyo metrics list
klaviyo profiles list --filter 'equals(email,"someone@example.com")'
klaviyo profiles get 01ABC123 --fields-profile email,first_name
klaviyo lists list --paginate          # follow cursors, merge all pages
klaviyo events create -d @event.json

# Or call any endpoint raw
klaviyo api /api/metrics/
klaviyo api POST /api/events/ -d @event.json
```

## Multiple accounts

Every stored key belongs to a named account profile:

```bash
klaviyo auth login --account prod
klaviyo auth login --account staging
klaviyo auth list                      # * marks the default
klaviyo auth switch staging            # change the default

# One-off override, highest to lowest precedence:
klaviyo api /api/metrics/ --account prod
KLAVIYO_ACCOUNT=prod klaviyo api /api/metrics/
```

Keys are stored in `~/.config/klaviyo/config.toml` with `0600` permissions (OS keychain storage is planned — see issue #1). For CI, skip stored accounts and set `KLAVIYO_API_KEY` (or pass `--api-key`).

## Commands

| Command | Description |
| --- | --- |
| `klaviyo auth login` | Store an API key for a named account |
| `klaviyo auth logout <account>` | Remove an account and its key |
| `klaviyo auth list` | List configured accounts |
| `klaviyo auth switch <account>` | Set the default account |
| `klaviyo auth status` | Verify credentials for the selected account |
| `klaviyo api [method] <path>` | Raw authenticated API request |
| `klaviyo version` | Print the CLI version |

Typed resource commands (`klaviyo profiles list`, `klaviyo campaigns get`, ...) are the next milestone — see [ARCHITECTURE.md](ARCHITECTURE.md).

## Development

Requires Go 1.25+; everything else self-installs.

```bash
make build   # builds bin/klaviyo
make test    # go test -race ./...
make lint    # installs the pinned golangci-lint into ./bin, then runs it
make fmt     # gofumpt + goimports via golangci-lint fmt
```

Releases are cut by pushing a `v*` tag; GoReleaser builds and publishes the binaries.

## License

[MIT](LICENSE)
