[English](README.md) | **简体中文**

<div align="center">

<img src="docs/images/icon.png" width="112" alt="magpie-local">

# magpie-local

### 一个网关，所有 Agent。只认路由组，顺序尝试。

**Claude Code、Codex、Gemini CLI、OpenCode…… 共用一个本地网关。**<br>
端口只认路由组的裸名；组内按保存顺序逐条尝试，报错才换下一条。

![Windows](https://img.shields.io/badge/Windows-x64-0078D4?logo=windows&logoColor=white)
![Go](https://img.shields.io/badge/Go-1.26.8-00ADD8?logo=go&logoColor=white)
![Wails](https://img.shields.io/badge/Wails-v3-DF0000)
![License](https://img.shields.io/badge/license-MIT-111111)
![Upstream](https://img.shields.io/badge/upstream-yetone%2Fmagpie-111111?logo=github)

</div>

<br>

> **[yetone/magpie](https://github.com/yetone/magpie) 的个人精简分叉。**不是官方项目，也不追求和上游功能对齐：
> 这里只有"路由组 + 顺序链"，界面是中文，上下文窗口不猜。MIT 许可，随便用。

## 为什么会有这个分叉

上游的 magpie 能力很全：竞速、钉住、冷却、多种路由模式、插件生态。但我自己用下来，真正需要的只有一件事——**给一个组名，让它按顺序把备选试一遍**。于是有了这个精简版：

| | |
| :-- | :-- |
| **只认组名** | 端口只接受路由组的裸名（大小写不敏感）。`provider/model` 直连、`group/<id>` 前缀一律 404。Agent 的模型名就是组名。 |
| **纯顺序链** | 每个请求完全独立，按组内保存顺序逐条尝试，只有上一条报错才换下一条。没有竞速、没有钉住、没有冷却，也没有按 key 的并发上限。 |
| **条目各自身份** | 每个请求内的条目都有独立编号。同一供应商、同一 key 的两条相邻条目，并发时日志也不会认错。 |
| **400 也往下走** | 组内条目返回普通 `400`（`The request is invalid` 这类）同样会尝试下一条，不会把整条链卡在第一条。 |
| **图片不预过滤** | 带图请求不再预先挑条目：图片原样发给每一条，上游以图片为由拒绝就自动换下一条。只有已知收不了图的条目，历史图片会被裁掉，当前图保留。 |
| **上下文不猜** | 只认供应商自己报的窗口。没报、或从未抓过清单，一律按 **200K**，不再用 models.dev 按模型 id 顶替别的部署的数字。 |

## 上下文窗口：200K 就是 200K

models.dev 给某个模型 id 的窗口，可能属于**另一个部署**。比如 orcarouter 的 `deepseek-v4.1-flash-free`，目录里只有 opencode 部署的 200K，而 orcarouter 自己其实是 1M——旧逻辑会把这个数字安到它头上。

现在规则很简单：

- 供应商清单（`/v1/models`、插件/账号清单）**报了多少就是多少**；
- 没报、或没有清单 → **200K**（`WindowUnknown`）；
- 路由的"上下文预跳过"**只认供应商真正报了的窗口**。默认的 200K 只用于显示和告知 Agent，不会把长请求提前跳过，仍由上游自己拒绝。

## 界面

中文侧边栏，五个页面。**供应商列表**和**条目列表**共用同一张板，区别只有一个：前者把 key 折叠掉。

### 供应商列表

按「供应商 × 模型」一行，不显示识别码和密钥。"原上下文长度"和"现上下文长度"两列并列：

- **原上下文长度**：上游自己报的窗口；没报显示 `—`。
- **现上下文长度**：实际生效的窗口（用户 > 上游 > 200K），单位 **K**，可直接编辑。

编辑规则也一眼能看懂：

| 情况 | 文本框 | 数字颜色 |
| :-- | :-- | :-- |
| 上游能拉到窗口 | **锁定**，不可改 | 常规色 |
| 上游拉不到，默认 200K | 可编辑 | **灰色** |
| 上游拉不到，用户设置的值 | 可编辑 | **绿色** |

回车或失焦保存，清空恢复 200K。保存走 `POST /api/provider/context`，和供应商编辑器写的是同一个 `contexts`，会同时通知 Agent。

### 条目列表

旧版供应商列表：按 key / 账号逐行展开，保留识别码与密钥两列。调度用的并发数已经搬到路由组页面，这里只看条目本身。

### 请求

实时在途请求 + 完整日志（不再只显示 10 条）。点开一行，展开整条尝试链：每个条目为什么失败、轮到哪一条、哪一条答了。同一时刻的并发请求顺序稳定，当日日志跨重启保留。

## 下载

预编译二进制都在 [Releases](https://github.com/hepeidong1/magpie-local/releases)。

| 平台 | 文件 | 说明 |
| :-- | :-- | :-- |
| Windows x64（桌面） | `magpie-local-windows-amd64.exe` | GUI，无命令行窗口 |
| Windows arm64（桌面） | `magpie-local-windows-arm64.exe` | GUI，无命令行窗口 |
| Windows x64 / arm64（CLI） | `magpie-local-cli-windows-amd64.exe` / `magpie-local-cli-windows-arm64.exe` | 仅终端 |
| macOS arm64 / x64（CLI） | `magpie-local-cli-darwin-arm64` / `magpie-local-cli-darwin-amd64` | 先 `chmod +x` |
| Linux x64 / arm64（CLI） | `magpie-local-cli-linux-amd64` / `magpie-local-cli-linux-arm64` | 先 `chmod +x` |
| Android / Termux arm64（CLI） | `magpie-local-cli-android-arm64` | 先 `chmod +x` |

`SHA256SUMS.txt` 是全部文件的校验和（`sha256sum -c SHA256SUMS.txt`）。桌面版目前只构建 Windows；其他平台用 CLI，或自行构建 GUI。

## 快速开始

**1 · 运行。** 从 Releases 下载 `magpie-local-windows-amd64.exe`（或 arm64 版），双击运行（Windows 10/11，GUI 子系统，无命令行窗口）。便携模式：配置写在同目录的 `data/`。

**2 · 加供应商。** 打开 **供应商登录**，选预设或填一个 OpenAI-compatible 地址，粘贴 API Key。

**3 · 建路由组。** 打开 **路由组**，新建一个组，把条目按你希望尝试的顺序排好。**组名就是 Agent 要填的模型名。**

**4 · 接 Agent。** Base URL 填 `http://127.0.0.1:3425/v1`（端口可在设置里改），模型名填**组名**。

**5 · 看请求。** 打开 **请求**，每次尝试了哪些条目、为什么失败，都在那里。

```text
Agent ── model: 主力 ──▶ 127.0.0.1:3425
                          │
                          ├─ 条目 1  供应商 A · 模型 X   ✗ 400  → 下一条
                          ├─ 条目 2  供应商 B · 模型 Y   ✗ 429  → 下一条
                          └─ 条目 3  供应商 C · 模型 Z   ✓ 200  → 回答
```

## 构建

要求 Go 1.26.8。

```sh
# Windows GUI（无控制台窗口）
go build -trimpath -ldflags="-H windowsgui" -o magpie-local.exe .

# 仅命令行版（可跨平台）
go build -tags nogui -o magpie .
```

测试与静态检查：

```sh
go test -tags nogui ./...
GOOS=linux  go vet -tags nogui ./...
GOOS=windows go vet ./...
```

也可以用仓库自带的 `make build` / `make test`。

## 目录

- `internal/gateway` — 网关：链式尝试、请求轨迹、日志
- `internal/provider` — 供应商、模型目录、路由组；上下文窗口解析在 `models.go`
- `internal/gui` — Wails v3 窗口与内嵌前端（`assets/`，纯 JS，无构建步骤）
- `internal/gui/assets/plugins` — 页面插件：entries（条目列表）、suppliers（供应商列表）、routing（路由组）、requests（请求）
- `plugins/` — 内置插件的 Go 侧入口
- `docs/` — 子系统设计文档与 README 图片
- `data/` — **首次运行才生成**，含全部配置、缓存和日志；不要提交

## 隐私与安全

- 这个包**不含** `data/`、密钥、账号、日志、缓存或任何已安装的第三方插件。
- 运行后请勿把 `data/`（尤其 `providers.json`、`caller-keys.json`、`logins.json`）上传或提交。
- 传到 GitHub 时，把二进制作为 **Release 附件**，不要提交进仓库（二进制进 Git 会让仓库很重；`.gitignore` 已忽略它们）。

## 已知限制

- 附带 exe 仅适用于 **Windows x64**；其他平台请自行构建。
- 个人自用分叉，按"现状"提供，不保证与上游行为一致；竞速、钉住、冷却等上游路由能力已被移除。
- models.dev 仍用于模型元信息和价格，但**不再兜底上下文窗口**。

## 许可

MIT，见 [LICENSE](LICENSE)。上游项目：[yetone/magpie](https://github.com/yetone/magpie)（Copyright (c) 2026 yetone）。第三方组件与许可见 [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES)。
