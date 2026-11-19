# Notes for coding agents

## What magpie is

magpie (module `github.com/yetone/magpie`) is a Go desktop app + local gateway
that lets you pick any model for any AI coding agent (Claude Code, Codex,
Gemini CLI, OpenCode, …). It edits each agent's own config, and one local
gateway serves all of them, moving to a next account or routing group when a
quota runs out. GUI via Wails v3 (`internal/gui`), terminal UI via
bubbletea (`internal/tui`), commands in `internal/cli` (the binary starts in
`main.go`). Builds for darwin/linux/windows, Termux (`android/…`, `nogui`),
and Docker.

### Major directories

- `internal/cli`, `internal/gateway`, `internal/provider`, `internal/plugin`,
  `internal/pluginrt` — CLI, the gateway, providers, and the plugin runtime.
- `internal/gui/assets` — the web GUI (plain `app.js` + `i18n.js`; no build
  step; the Playwright regressions live in `internal/gui/tests`). `plugins/`
  — built-in plugins registered with `pluginrt`; the plugin host is
  `internal/plugin` (`host.js`).
- `docs/subsystems` — one design reference per subsystem (read before editing
  that subsystem). `docs/code-standards.md` — review rules and acceptance
  criteria. `LESSONS.md` — what recent merged work got wrong.

## Build, test, dev

Run these in magpie's own folder (the Makefile's `clean` recipes name paths
relative to the working folder and act there otherwise; use `make -C`).

- `make build` — the app (platform tags, cgo webview).
- `make cli` — terminal-only `magpie`, cross-compiles (`-tags nogui`).
- `make test` — `go vet` + `go test` with the app tags.
- `make test-ui` — the GUI regressions under `internal/gui/tests` (Playwright,
  Chromium and WebKit; run locally, CI does not run this suite).
- `make dev` — hot-reload dev (`build/dev.sh`, needs `fswatch`), else
  `make dev-once`. Uses its own ports (`127.0.0.1:3426`–`3429`).
- `make release-windows` / `release-linux` / `release` — packaged artifacts.

Full acceptance criteria for a change are in `docs/code-standards.md` (sandbox
tests, GUI rules, commit record). Go conventions there: `any` over
`interface{}`, `appdir.Getenv` for folder env vars, no blocking calls under a
lock the reader takes, failed reads parse as "unknown" not "empty".

## Testing rules

- Run the whole suite (`go test -tags nogui ./...`), not just the packages
  you touched: the guard tests live elsewhere — `TestFolderVarsAreListed`
  (`internal/agent`) fails when a new agent variable isn't in
  `agentenv.Vars`, `TestNoCommandBypassesProc` (`internal/proc`) when code
  calls `exec.Command` instead of `proc.Command`.
- Tests never reach the network (vendors are served from `httptest` built
  on saved bytes) and never touch a live agent config (`~/.codex`,
  `~/.claude`, `~/.gemini`, …) or `~/.config/magpie`. Run them under a temp
  HOME with `internal/testenv` (marker `MAGPIE_TEST_SANDBOX`); the shell
  snippet pins `GOPATH`/`GOMODCACHE`/`GOCACHE` before swapping HOME
  (`docs/subsystems/provider-plugins.md`, Verification).
- A new agent environment variable goes in `agentenv.Vars` (or
  `agentenv.NotPaths` when it names no folder), so the sandbox clears it.
- Acceptance also runs cross-OS vet — `GOOS=linux go vet -tags nogui ./...`,
  `GOOS=windows go vet ./...` — plus the matching builds; vet compiles
  `_test.go` files, a cross build doesn't. Run any change that starts a
  goroutine or timer under `-race`.
- Some plugin tests need Bun on PATH and skip when it is missing; report
  skipped tests.

## Subsystem design and review

Before changing a documented subsystem, read its
[design reference](docs/subsystems/README.md) and verify the linked source.
When preparing or reviewing a PR, follow the same page's documentation update,
semantic change description, and review rules.

## Code standards

Write and review changes against the [code standards](docs/code-standards.md):
a change does only what its goal needs; a fix reaches every sibling of the
bug (grep the same pattern); a fix comes with a test that fails without it;
a magpie default never overrides a user-set value; one commit has one goal,
with what was run recorded in the body. GUI conventions: every user-visible
string goes through `t()` with translations in `i18n.js` (tests run in
Chinese, English, zh-TW, ja, de); no native `<select>` — use the app's own
menu (`openProtoMenu`); a click never scrolls the page (`scrollOnPurpose`);
no colored left-border stripes — mark state with a dot or a swatch.

## Community PRs

Before reviewing a community PR, read [contributors](docs/contributors.md):
what stops a review for possible harm, what is closed unreviewed, and the
authors whose PRs are closed on sight.

## Lessons

Before changing code, read [LESSONS.md](LESSONS.md): what recent merged
work got wrong and the rule that would have caught it.

## Built-in subscriptions that a plugin serves

These built-in subscriptions are retired. Reaching them can break their
vendors' terms, so they moved out of magpie into community OpenCode plugins,
which keeps magpie itself from being banned. Each one has a plugin that does
the same job:

| Subscription | id | Plugin (npm) | Mover |
| --- | --- | --- | --- |
| Command Code | `commandcode-plan` | `@magpie-community/opencode-commandcode-auth` | `internal/provider/migrate_side.go` |
| Cursor | `cursor` | `@magpie-community/opencode-cursor-auth` | `internal/provider/migrate_side.go` |
| Devin | `devin` | `@magpie-community/opencode-devin-auth` | `internal/provider/migrate_side.go` |
| Factory | `factory` | `@magpie-community/opencode-factory-auth` | `internal/provider/migrate_factory.go` |
| Grok | `grok` | `@magpie-community/opencode-grok-auth` | `internal/provider/migrate_side.go` |
| Kiro | `kiro` | `@magpie-community/opencode-kiro-auth` | `internal/provider/migrate_kiro.go` |
| Xiaomi MiMo | `mimo-app` | `@magpie-community/opencode-mimo-auth` | `internal/provider/migrate_mimo.go` |
| Qoder, Qoder CN | `qoder`, `qoder-cn` | `@magpie-community/opencode-qoder-auth` | `internal/provider/migrate_qoder.go` |
| WorkBuddy, WorkBuddy AI | `workbuddy`, `workbuddy-ai` | `@magpie-community/opencode-workbuddy-auth` | `internal/provider/migrate_workbuddy.go` |
| ZCode | `zcode` | `@magpie-community/opencode-zcode-auth` | `internal/provider/migrate_zcode.go` |
| Zed | `zed` | `@magpie-community/opencode-zed-auth` | `internal/provider/migrate_zed.go` |

The `movers` map in `internal/provider/migrate*.go` is the source of truth.
`TestMovedBuiltinsSayTheirPlugin` fails when a mover has no notice in its
code.

### Which code actually runs

Two cases decide it:

- **Moved:** the user moved the subscription onto its plugin, from the
  editor's "Move to plugin" (or `provider.Adopt`). Its `migrations.json`
  state is `plugin` (`provider.Moved(id)`). A new user never sees a
  deprecated built-in: the Add sheet leaves one with no account out
  (`unusedSub` in app.js), so it is installed from the Plugins page. A user
  who installs the plugin themselves is moved too
  (`provider.HandOver`): at once when the built-in has no accounts, from the
  gateway's hourly loop (`KeepRetiringMoved`) when it has, unless the
  plugin is signed in under its own `-plugin` id already. Until
  then the Add sheet shows the plugin's tile only (`replacedSub` in app.js).
  Since 10-16 the built-ins are being retired: the hourly loop's
  `MoveRetiring` moves every subscription with a mover that still has
  built-in accounts, a user who moved back included, and there is no way
  back. Turning the plugin off turns the subscription off with it; removing
  it records `removed` (accounts kept with the plugin's sign-ins), and
  installing it again gives the subscription back.
- **Not moved:** the user still has accounts from the built-in: the move
  hasn't run yet, or it failed (tried again after six hours). Nothing
  serves them until it runs.
- **Retired:** the built-in's code is gone (`retiredBuiltins` in
  `internal/provider/migrate.go`; `provider.BuiltinRetired(id)`). The
  plugin serves it, moved or not. Accounts not moved yet are served by
  nothing; a line over the Providers list says so, with why and a Move
  (`providers.unmoved`, `unmovedLine` in app.js). With no row of its own,
  the Add sheet leaves it to the Plugins page (`unusedSub`). Every
  subscription in the table above is retired (10-17); gates ask
  `provider.PluginServes(id)` (moved or retired).

The plugin does everything: sign-in, refresh, models, requests, usage and
errors. It runs in the plugin host (`internal/plugin`). The built-ins'
sign-in, models, usage and gateway translators are gone. What is left of
them in `internal/provider` (and `internal/qoder/endpoints.go`) reads the
built-in's saved accounts so the mover can carry them over. The gateway
sends every request to the plugin's fetch.

So a bug report about one of these subscriptions is usually about the
plugin:

1. Check whether the user's subscription is moved. A moved provider's row in
   `/api/providers` has `move.state == "plugin"`; the provider id is in
   `onPlugins`.
2. Fix the plugin in the community repo,
   [magpie-community/plugins](https://github.com/magpie-community/plugins)
   (`packages/<name>`, provider id = the built-in's id; checked out locally
   at `~/workspace/projects/magpie-commuity-plugins`). Publish a new version.
3. Raise the mover's `min` to that version. `keepMovedCurrent` then updates
   every moved user's plugin.

This magpie-side code still runs for moved subscriptions:

- the plugin host: `internal/plugin`, `host.js`;
- `internal/provider/plugins.go`, `plugin_usage.go` (`movedCards`) and
  `pluginsignin.go`;
- the move itself: `internal/provider/migrate*.go`;
- the GUI's plugin paths in `internal/gui/assets/app.js`: `subOf`,
  `pluginSubs` and `startPluginSignIn`.

The parity tests, `internal/gateway/plugin_*_test.go`, hold each plugin to
what its built-in did.
