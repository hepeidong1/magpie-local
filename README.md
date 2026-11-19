**English** | [简体中文](README.zh-CN.md)

<div align="center">

<img src="docs/images/icon.png" width="112" alt="magpie-local">

# magpie-local

### One gateway for every agent. Routing groups only, tried in order.

**Claude Code, Codex, Gemini CLI, OpenCode… sharing one local gateway.**<br>
The port accepts nothing but a routing group's bare name; the group's entries are tried in saved order, and it moves on only when one fails.

![Windows](https://img.shields.io/badge/Windows-x64-0078D4?logo=windows&logoColor=white)
![Go](https://img.shields.io/badge/Go-1.26.8-00ADD8?logo=go&logoColor=white)
![Wails](https://img.shields.io/badge/Wails-v3-DF0000)
![License](https://img.shields.io/badge/license-MIT-111111)
![Upstream](https://img.shields.io/badge/upstream-yetone%2Fmagpie-111111?logo=github)

</div>

<br>

> **A personal, trimmed-down fork of [yetone/magpie](https://github.com/yetone/magpie).** Not an official project, and not trying to match upstream feature for feature:
> just "routing groups + a sequential chain", a Chinese UI, and no guessing at context windows. MIT licensed — use it as you like.

## Why this fork exists

Upstream magpie is feature-rich: races, pinning, cooldowns, several routing modes, a plugin ecosystem. In daily use the only thing I actually needed was one thing — **give it a group name and let it walk the alternatives in order.** So this lite build exists:

| | |
| :-- | :-- |
| **Group names only** | The port accepts a routing group's bare name (case-insensitive). `provider/model` and `group/<id>` are 404. The model name an agent uses is the group name. |
| **Sequential only** | Every request is independent: entries are tried in saved order, and the next is asked only after the previous one errors. No races, no pinning, no cooldowns, no per-key concurrency caps. |
| **Per-entry identity** | Entries within a request are numbered. Two adjacent entries of the same provider and key are never mis-attributed in the log, even under concurrency. |
| **Plain 400s move on** | A plain `400` from an entry (the "The request is invalid" kind) still advances to the next entry; the chain doesn't stop at the first 400. |
| **Images are not pre-filtered** | Image requests are sent to every entry as-is; if the upstream refuses the image, the chain moves on. For entries known to be text-only, images in earlier turns are stripped; the current image is kept. |
| **No context guessing** | Only windows the provider itself reports. If it reports none — or no list was ever fetched — a fixed **200K** is used, never another deployment's number from models.dev. |

## Context windows: 200K means 200K

A window models.dev gives for a model id may belong to **another deployment**. For example, for orcarouter's `deepseek-v4.1-flash-free` the directory only has opencode's 200K, while orcarouter's own is 1M — the old logic pinned that number on it.

The rules are simple now:

- What the provider's own list (`/v1/models`, a plugin's or account's list) **reports is used as-is**;
- Nothing reported, or no list → **200K** (`WindowUnknown`);
- Routing's "context pre-skip" **only honors a window the provider actually reported**. The 200K default is only for display and for telling agents; it never pre-skips a long request — the upstream itself refuses it if it can't take it.

## UI

A Chinese sidebar with five pages. **Provider list** and **Entry list** share one board; the only difference is that the former folds keys away.

### Provider list (供应商列表)

One row per provider × model, with no entry code and no key. "Original window" and "Current window" sit side by side:

- **Original window**: what the upstream itself reported; `—` when it reported none.
- **Current window**: the window in effect (user > upstream > 200K), unit **K**, editable in place.

The editing rules are easy to read:

| Case | Box | Number colour |
| :-- | :-- | :-- |
| Upstream reports a window | **Locked**, read-only | normal |
| Not reported, default 200K | editable | **grey** |
| Not reported, user-set value | editable | **green** |

Enter or blur saves; clearing restores 200K. Saves go through `POST /api/provider/context`, writing the same `contexts` as the provider editor, and notify agents at the same time.

### Entry list (条目列表)

The old provider list: one row per key/account, keeping the entry-code and key columns. Scheduling concurrency has moved to the routing-group page; here you only see the entry itself.

### Requests (请求)

Live in-flight requests plus the full log (no longer only the first 10 rows). Open a row to see the whole attempt chain: why each entry failed, which one it was on, which one answered. Concurrent requests heard at the same instant keep a stable order, and the day's log survives a restart.

## Quick start

**1 · Run.** Unzip anywhere and double-click `magpie-local.exe` (Windows 10/11 x64, GUI subsystem, no console window). Portable: configuration lives in `data/` next to the exe.

**2 · Add a provider.** Open **供应商登录** (Provider logins), pick a preset or enter an OpenAI-compatible base URL, and paste an API key.

**3 · Build a routing group.** Open **路由组** (Routing groups), create a group, and order its entries the way you want them tried. **The group name is the model name.**

**4 · Connect an agent.** Base URL: `http://127.0.0.1:3425/v1` (the port is configurable in Settings); model: the **group name**.

**5 · Watch requests.** Open **请求** (Requests) to see which entries each request tried and why they failed.

```text
Agent ── model: main ──▶ 127.0.0.1:3425
                         │
                         ├─ entry 1  provider A · model X   ✗ 400  → next
                         ├─ entry 2  provider B · model Y   ✗ 429  → next
                         └─ entry 3  provider C · model Z   ✓ 200  → answer
```

## Build

Requires Go 1.26.8.

```sh
# Windows GUI (no console window)
go build -trimpath -ldflags="-H windowsgui" -o magpie-local.exe .

# CLI only (cross-platform)
go build -tags nogui -o magpie .
```

Tests and static checks:

```sh
go test -tags nogui ./...
GOOS=linux  go vet -tags nogui ./...
GOOS=windows go vet ./...
```

The repo's `make build` / `make test` work too.

## Layout

- `internal/gateway` — the gateway: chain attempts, request traces, logs
- `internal/provider` — providers, the model catalog, routing groups; context-window resolution lives in `models.go`
- `internal/gui` — Wails v3 window and the embedded frontend (`assets/`, plain JS, no build step)
- `internal/gui/assets/plugins` — page plugins: entries (Entry list), suppliers (Provider list), routing (Routing groups), requests (Requests)
- `plugins/` — Go-side entry points of the built-in plugins
- `docs/` — subsystem design docs and README images
- `data/` — created on first run; holds all configuration, caches and logs; do not commit it

## Privacy and safety

- This package contains **no** `data/`, keys, accounts, logs, caches or installed third-party plugins.
- After running it, never upload or commit `data/` (especially `providers.json`, `caller-keys.json`, `logins.json`).
- For GitHub, attach `magpie-local.exe` to a **Release** rather than committing it (77 MB; binaries make a repo heavy; `.gitignore` already ignores it).

## Known limitations

- The bundled exe is **Windows x64 only**; other platforms need their own build.
- A personal fork, provided as-is, with no guarantee of matching upstream behaviour; upstream routing features such as races, pinning and cooldowns have been removed.
- models.dev is still used for model metadata and prices, but **no longer as a context-window fallback**.

## License

MIT — see [LICENSE](LICENSE). Upstream: [yetone/magpie](https://github.com/yetone/magpie) (Copyright (c) 2026 yetone). Third-party components and licenses: [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).
