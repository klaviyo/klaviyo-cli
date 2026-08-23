# CLI product bootstrap tracker

Lightweight work tracker for the initial build. Temporary: delete when this
phase is done and move remaining work to GitHub issues. Checked items are
done and verified; items marked `(review)` need a decision review before
implementation.

## Done

- [x] Repo scaffold: Go 1.25 + Cobra, `internal/` layout, module `github.com/klaviyo/klaviyo-cli`
- [x] Multi-account auth: named profiles, `auth login/logout/list/switch/status`, precedence chain (flag > env > profile)
- [x] Key storage in config.toml (0600); keychain migration tracked in [#1](https://github.com/klaviyo/klaviyo-cli/issues/1)
- [x] Raw API escape hatch: `klaviyo api [method] <path>` with `-q` params, `-d` body (`@file`/stdin), revision pinning + `--revision`, 429 retry with Retry-After
- [x] CI: pinned self-installing golangci-lint via `make lint`, `go test -race` on 3 OSes, GoReleaser snapshot; releases on `v*` tags

## Decision: how typed commands stay current with the API

**Chosen: build-time generation from the OpenAPI spec (the Stripe model), not runtime interpretation.**

How Stripe does it (confirmed from their repo): the OpenAPI spec is *committed
into the CLI repo*; `go generate` produces compiled-in command specs (~52k
lines generated); a GitHub Actions workflow (`sync-openapi-artifacts.yml`)
downloads the new spec on demand, reruns generation, and opens a PR; releases
are frequent (10 in a recent 3-week stretch) so the gap between "API shipped"
and "CLI knows it" is days. GitHub's CLI is the counterexample: fully
hand-written commands, no spec — viable only because they curate a UX-first
subset. Runtime interpretation (fetch spec on demand, e.g. restish) maximizes
freshness but costs startup latency, offline help, shell completion of
commands, and UX stability.

Why build-time for us: static binary stays self-contained; help/completions
work offline; generated commands get reviewed in a PR before shipping; and the
`api` command + `--revision` already covers brand-new endpoints between
releases.

- [x] Vendor `openapi/stable.json` into the repo (pin the exact revision the build was generated from)
- [x] Generator (`internal/gen`): spec → `internal/cli/resources_gen.go` + generated `DefaultRevision`
- [x] Generic executor: builds all 345 commands from generated specs (path params positional, query params as flags, `-d` body, `--paginate` merges cursor pages)
- [x] CI guard: `go generate` + `git diff --exit-code` so generated code can't drift from the vendored spec
- [x] `sync-openapi.yml` workflow: weekday schedule + manual; fetch klaviyo/openapi, regenerate, open PR when changed
- [ ] Release checklist: merged sync PR → tag → GoReleaser

## Command map (spec revision 2026-07-15: 345 operations, 23 tags)

One command group per spec tag. Group name = lowercased tag.

| Group | Ops | Group | Ops |
|---|---|---|---|
| `catalogs` | 55 | `lists` | 13 |
| `custom-objects` | 39 | `client` | 12 |
| `profiles` | 38 | `templates` | 12 |
| `campaigns` | 26 | `segments` | 11 |
| `tags` | 26 | `forms` | 9 |
| `metrics` | 24 | `events` | 8 |
| `flows` | 21 | `reporting` | 7 |
| `coupons` | 17 | `webhooks` | 7 |
| `images` | 5 | `web-feeds` | 5 |
| `reviews` | 3 | `tracking-settings` | 3 |
| `accounts` | 2 | `conversations` | 1 |
| `data-privacy` | 1 | | |

Mapping rules (operationIds are regular enough to be rule-based):

| operationId pattern | Command |
|---|---|
| `get_<plural>` | `klaviyo <group> list` |
| `get_<singular>` | `klaviyo <group> get <id>` |
| `create_<singular>` | `klaviyo <group> create` |
| `update_<singular>` | `klaviyo <group> update <id>` |
| `delete_<singular>` | `klaviyo <group> delete <id>` |
| `get_<rel>_for_<res>` | `klaviyo <group> <rel> <id>` (e.g. `profiles lists <id>`) |
| everything else | kebab-case of operationId minus redundant prefix (e.g. `bulk_import_profiles` → `profiles bulk-import`) |

Open design decisions:

- [ ] (review) Relationship-ID endpoints (`get_<rel>_ids_for_<res>`, ~70 ops): separate commands vs an `--ids-only` flag on the relation command
- [ ] (review) `client` group: public client-side endpoints authed by public key/company ID, not private key — expose, or skip in v1?
- [ ] (review) Sub-resources spanning tags (e.g. `campaign-messages` under Campaigns): nested group (`campaigns messages get`) vs flattened commands
- [ ] (review) Common flags for generated commands: `--filter`, `--sort`, `--fields`, `--include` (JSON:API params) as first-class flags vs raw `-q`

## Build list: features both Stripe CLI and GitHub CLI have

Assumed in scope (per review 2026-08-23). Ordered roughly by dependency.

- [x] Multi-account/profile auth with switch (`gh auth switch` / stripe `--project-name` + `switch`)
- [x] Raw API access (`gh api` / stripe `get|post|delete`)
- [x] Typed resource commands (stripe: generated; gh: hand-written — we generate, see above). v1 note: bodies are raw `-d` JSON, not typed per-field flags like Stripe generates; open decisions on relationship/`client` command curation below still stand (all currently exposed uniformly)
- [x] Shell completions: Cobra `completion` command plus dynamic account-name completion for `--account`, `auth switch`, `auth logout`
- [ ] `config` command (get/set/list; open in editor)
- [x] `--jq` global flag (embedded gojq, no external jq) and `--paginate`; TTY-aware table output still open
- [x] Update notifier: background check of latest GitHub release max once/24h (cached in `update-check.json`), stderr notice, `KLAVIYO_NO_UPDATE_NOTIFIER`/CI/non-TTY guards. NOTE: no-ops until the repo is public (unauthenticated Releases API)
- [ ] Open-in-browser (`gh browse` / `stripe open`): `klaviyo open <shortcut>` → dashboard/docs deep links
- [ ] Webhook forwarding to localhost (stripe `listen` core / gh `webhook forward` official extension): `klaviyo listen` — needs a relay or polling design against Klaviyo webhooks
- [ ] Browser-based login (stripe pairing-code flow / gh device flow) — depends on OAuth support; today's API-key login is the interim
- [ ] Docs-from-code: generated command reference (gh generates its manual from the Cobra tree) — seeds the docs site

## Review list: features one CLI has but not the other

Each needs a keep/skip decision. Notes give the Klaviyo analog.

**Stripe-only:**

- [ ] `trigger` — fire sample webhook/API events for testing. Klaviyo analog: create test events/profiles against a test account. Pairs with `listen`.
- [ ] `fixtures` — JSON-described sequences of dependent API calls (seed data, flows). Strong analog for seeding test accounts (profiles → lists → events).
- [ ] `logs tail` — real-time API request log streaming. Klaviyo has no public request-log stream; nearest analog is polling `events`. Skip or reshape as `events tail`.
- [ ] `samples` — scaffold sample integrations from template repos.
- [ ] `sandbox` — provision test sandboxes from the CLI (Stripe even pre-account). Analog: Klaviyo test accounts.
- [ ] `agent setup` — installs MCP server/skills for AI agents. Klaviyo *has* an MCP server; `klaviyo agent setup` could configure it. Timely.
- [ ] Plugin system (hidden; hashicorp go-plugin, checksum-verified, Stripe-distributed manifests) — vs gh's open extension model below; pick at most one, later.
- [ ] Live/test mode key slots per profile — Klaviyo has no live/test key split; likely N/A (test accounts are separate accounts = separate profiles). Probably skip.
- [ ] `serve`, `terminal`, `docs search`, `feedback`, `community` — low value for v1; default skip.

**GitHub-only:**

- [ ] `alias` — user-defined command shortcuts (incl. shell aliases).
- [ ] Extension system — any repo named `klaviyo-cli-<x>` becomes `klaviyo <x>`; open distribution, binary or script, update notices. (vs Stripe's closed plugin system.)
- [ ] `--json`/`--template` rich formatting beyond `--jq` (Go templates with helpers).
- [ ] Interactive TTY prompting/TUI (pickers, editors; `GH_PROMPT_DISABLED` escape). We have minimal prompting in `auth login` already; decide how far to go.
- [ ] `search` — cross-resource search commands. Klaviyo analog: wrappers over JSON:API `filter` (e.g. `profiles search --email x`). Could be sugar on generated commands.
- [ ] `status` — cross-cutting "what needs my attention" summary. Klaviyo analog: recent campaign performance/flow errors digest. Product idea, not v1.
- [ ] `secret`/`variable`, `ssh-key`, `attestation`, `copilot` launcher — GitHub-domain-specific; N/A.

## Implementation review queue

Decisions to walk through together before/as each lands (same format as the
scaffold review):

- [ ] Generator design: template-based codegen vs generic executor + data tables (Stripe uses both)
- [ ] Pagination + output UX for `list` commands
- [ ] Update notifier mechanics (release lookup on a private repo won't work until public)
- [ ] `listen` architecture (Klaviyo webhooks have no CLI relay service today — needs design)
- [ ] Docs site tooling
