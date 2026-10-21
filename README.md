<p align="center">
  <img src="logo.png" width="200" alt="Gemini Free">
</p>

<h1 align="center">Gemini Free</h1>

Go + MyGo 桌面客户端，支持 macOS 和 Windows，将 Gemini 网页端转换为本地 OpenAI / Anthropic Messages 兼容 API。提供完整 GUI、菜单栏／托盘右键菜单、匿名与登录会话模式。

基于 [gemini-web2api](https://github.com/Sophomoresty/gemini-web2api) 的协议逆向逻辑重写。

## Go 桌面版

构建产物位于 `dist/`：

- macOS：`dist/darwin-universal/Gemini Free.app`，兼容 Apple Silicon 和 Intel。
- Windows：`dist/windows-amd64/Gemini Free.exe` 及同目录安装程序。

[图文使用教程](docs/tutorial/index.html) · [自动化测试报告](artifacts/automation-20261021/REPORT.md) · [2026-10-21 代码审查](artifacts/review-20261021/REPORT.md)

打开客户端后，首页可启停服务、切换请求模式及复制兼容接口地址。问答页会显示真实上游路由和思考等级；设置中管理登录会话、代理、监听地址和 API 访问密钥。关闭窗口后应用继续在托盘运行，从托盘选择退出才停止服务。

匿名模式不读取账号 Cookie，不携带登录 token。2026-10-21，Go 版本已完成真实问答和增量输出验证，`gemini-flash-lite` 请求与实际路由均为 `6`。高级模型必须显式选择登录会话；权限不足或上游返回低档时明确报错。

登录会话支持 Cookie 文本或 JSON 导入，也可从旧版配置导入。Cookie、会话参数和 API 密钥以 AES-GCM 加密保存，主密钥由 macOS 钥匙串／Windows 系统凭据库管理。Go 版配置独立保存，不覆盖 Swift 版 Cookie 文件；导入不会自动切换模式。账号导入成功不代表高级模型权限已验证。

新增会话时仅 Cookie 为必填项，也可选择 Cookie 导出 JSON 或会话文件。名称留空时自动命名；XSRF token 和网页构建标识由会话刷新自动获取，高级参数默认折叠。关闭自动刷新时，需主动刷新会话以同步参数。

Go 版默认仅本机访问。首页“局域网设置”或设置中的“通用连接”提供局域网访问开关；停止服务后更改并保存，再启动服务即可生效。首页及设置显示各网卡的 OpenAI / Anthropic 地址并支持复制。局域网只开放兼容 API，管理界面仍仅本机可用；访问认证状态及密钥入口在同一设置页。

开发启动与检查：

```bash
go run ./cmd/gemini-free
go test -race -timeout 60s ./internal/...
go vet ./...
```

需要代理时可使用 `go run ./cmd/gemini-free -proxy http://127.0.0.1:7890`。`-config` 可指定独立配置路径，`-no-start` 使客户端启动时不自动开启兼容 API。

双端打包：

```bash
bash scripts/build-desktop.sh
```

当前使用 Go 1.27.1 和 MyGo v0.6.0。macOS 产物为本地 ad-hoc 签名，尚未进行 Apple 公证；Windows 产物尚未配置发行者签名。UI 证据及组件规范见 [设计分析](artifacts/autotyper-ui-20261021/COMPONENTS.md) 和 [设计规范](DESIGN.md)。

以下保留 Swift 旧版的使用和构建说明。旧版默认模型、监听地址及配置位置与 Go 版分别维护。

## Swift 旧版

## 下载

[Releases](../../releases/latest) 页有两种包：`GeminiFree.dmg`（拖入 Applications 安装，推荐）或 `GeminiFree-macOS.zip`。通用二进制，Intel 和 Apple 芯片的 Mac 都能跑。

## 用法

双击 `Gemini Free.app`，菜单栏出现图标，服务起在 `localhost:8081`。

客户端填：Base URL `http://localhost:8081/v1`，Model 填 `gemini-4-argon`。Argon 需要已登录、具备对应模型权限的 Google 账号，在设置中配置 Cookie 文件；不使用 Argon 时可选择下方的 Flash 模型。

```bash
curl http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gemini-4-argon","messages":[{"role":"user","content":"你好"}]}'
```

流式输出需在请求中设置 `"stream": true`：

```bash
curl -N http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gemini-4-argon","stream":true,"messages":[{"role":"user","content":"你好"}]}'
```

## Claude Code 与工具调用

Gemini Free 实现了 Claude Code 使用的 `POST /v1/messages` 与 `POST /v1/messages/count_tokens`。Claude Code 发来的 `input_schema` 会转换为 Gemini 网页端可理解的本地工具协议；Gemini 返回的调用会转换为 Anthropic `tool_use` 内容块，Claude Code 执行工具后发回的 `tool_result` 也会进入下一轮上下文。

先启动 Gemini Free，然后从同一个终端启动 Claude Code：

```bash
export ANTHROPIC_BASE_URL=http://127.0.0.1:8081
export ANTHROPIC_AUTH_TOKEN=gemini-free
export ANTHROPIC_CUSTOM_MODEL_OPTION=gemini-4-argon
export ANTHROPIC_CUSTOM_MODEL_OPTION_NAME="Gemini 4 Argon (Gemini Free)"
export ANTHROPIC_MODEL=gemini-4-argon
export ANTHROPIC_DEFAULT_MODEL=gemini-4-argon
claude
```

如果在 Gemini Free 设置里配置了 API Key，把 `gemini-free` 换成其中一个 Key；未配置时该占位 token 仅用于让 Claude Code 启用网关模式。可在 Claude Code 的 `/status` 中确认 `Anthropic base URL` 指向本机。

工具请求支持非流式响应和 Anthropic SSE 事件序列，包括 `tool_use`、`input_json_delta`、`tool_result` 续轮与 `stop_reason: tool_use`。由于 Gemini 网页接口没有公开任意函数声明能力，本项目通过严格的结构化提示词实现工具调用；带工具的响应需等模型完整生成后才能解析。模型生成畸形参数或调用未声明工具时，接口会明确返回 `502`，不会把错误伪装成普通文本。

普通对话会以 OpenAI 兼容的 SSE 增量返回。工具调用需要先完整解析模型生成的 `tool_call` 块，因此可能在生成完成后才返回；客户端固定传入空的 `"tools": []` 不会影响普通对话的流式输出。

首次打开若提示"已损坏"或无法验证开发者，在终端跑一次：

```bash
xattr -dr com.apple.quarantine "Gemini Free.app"
```

（App 用 ad-hoc 签名、未做 Apple 公证，这是正常现象，跑一次即可。）

## 模型

Gemini 4 Argon：`gemini-4-argon`（默认高档）、`gemini-4-argon-low`、`gemini-4-argon-medium` 与 `gemini-4-argon-high` 已通过真实 API 测试。需配置具有 Argon 权限的有效登录 Cookie；自动续期会获取同一网页会话的 `xsrf_token`（`SNlM0e`）和 `gemini_bl`（`cfb2h`），关闭续期时需手工配置这些参数。客户端同时支持 OpenAI 与 Anthropic 接口。应用会发送独立的 Argon 模型路由请求头，并核对上游实际返回的模型与思考等级；权限不足、会话失效导致降级时明确报错，不会把 Flash 回复当成 Argon。

截至 2026-10-20，低、中、高三档均已核对上游响应元数据。过期或混用的会话参数可能使上游返回低档；1.8 起自动续期并同步 Cookie、`xsrf_token` 与 `gemini_bl`，继续严格核对响应档位。

Argon 思考等级可用别名选择，也可使用 `@think=1`（低）、`@think=4`（中）、`@think=2`（高），例如 `gemini-4-argon@think=4`。这是网页端的新思考等级枚举，与旧模型的深度参数不同。

免登录：`gemini-3.8-flash` `gemini-3.7-flash` `gemini-3.6-flash` `gemini-3.5-flash` `gemini-3.5-flash-thinking` `gemini-3.5-flash-thinking-lite` `gemini-auto` `gemini-flash-lite`

要 Gemini Advanced 付费 cookie：`gemini-3.1-pro` `gemini-3.1-pro-enhanced`（没 cookie 也能调，静默回退 Flash）

旧模型保留 `@think=N` 兼容参数，沿用原有编码；Argon 的思考等级使用独立字段。

### 匿名模式实测

2026-10-21，通过不读取本机账号配置、不携带 Cookie、Authorization 或登录 token 的独立进程，已验证普通问答、中文计算题和流式输出可用。使用本机代理完成测试，直接连接在建立连接时超时。

`gemini-flash-lite` 请求的路由编号为 `6`，本次上游也返回 `6`。`gemini-3.8-flash` 请求的路由编号为 `1`，本次上游实际返回 `6`；因此上述模型名称属于本地别名，不能仅凭请求名称确认实际模型或版本。匿名测试不能作为高级模型权限证明。

独立测试入口为 `Tests/AnonymousLiveTests.swift`，详细结果见 [匿名测试记录](artifacts/anonymous-20261021/RESULTS.md)。该测试不会读取或更新已有账号 Cookie。

## 设置

点菜单栏图标 → 设置，可改端口、局域网开关、API Key 鉴权、Cookie 文件、代理、默认模型。配置存 `~/.config/gemini-web2api/config.json`，格式和原项目通用。

- **局域网访问**：默认开启（监听 0.0.0.0），同网段设备可用本机 IP 调用；关闭则仅本机（127.0.0.1）。
- **API Key 鉴权**：填一个或多个 Key（逗号分隔），客户端需带 `Authorization: Bearer <key>`；留空则免密。开放局域网时建议设置。

代理填 `http://127.0.0.1:7890` 这类 HTTP 代理即可。

### 登录会话自动续期

1.8 默认启用自动续期：启动时、后台以及请求前检查，约每 3 分钟续期一次。后台每 30 秒检查到期状态，避免错过到期点。需要配置含 `__Secure-1PSID` 的有效 Google 登录 Cookie；匿名调用不执行续期。同一时刻只续期一次，长时间生成不阻塞续期；旧请求返回的 Cookie 不会覆盖已经轮换的新会话。

程序通过 Google 的 `RotateCookies` 续期登录，确认收到有效续期 Cookie 后立即原子写回原文件，再访问对应账号的 Gemini 页面更新 `xsrf_token` 和 `gemini_bl`。生成接口返回的 Cookie 也会保存。所有 Google 请求使用一致的 User-Agent 和设置中的代理。

Cookie 文件继续支持纯文本和 JSON 输入；首次续期会保存为 JSON，保留原有 Cookie 和 JSON 自定义字段，并设为仅当前用户可读写（`0600`）。自动获取的会话参数保存在这个文件中，优先于配置中的手工值，重启后继续使用。若网页参数获取失败，已轮换的 Cookie 仍会保留，下次继续获取参数。

设置中的“登录续期”可启用或关闭自动续期；配置项为 `session_auto_refresh`，默认为 `true`。菜单栏显示续期状态，支持“立即续期登录会话”。续期被拒绝、网页不再包含登录参数或文件无法保存时会明确报错，自动续期不能代替重新登录，也不会降低模型或思考档位。

登录 Cookie 已被 Google 撤销时，需要本人在浏览器完成密码、通行密钥或其他身份验证，再重新导出 Cookie。旧标签仍显示对话不代表登录仍有效，可打开新的 Gemini 页面确认。部分浏览器的设备绑定会话可能拒绝独立程序续期；程序会显示实际失败，不会自动改用浏览器桥接或其他来源。当前版本不保证这类 Cookie 可脱离浏览器长期续期。

## 编译

```bash
./build.sh
```

运行回归测试：`bash test.sh`，每个测试进程限时 60 秒。已配置本机 Cookie 后，可手动测试真实 Argon 路由：

```bash
swiftc Sources/Config.swift Sources/Engine.swift Sources/GeminiPayload.swift \
  Sources/GeminiResponse.swift Sources/Models.swift Sources/Util.swift \
  Sources/GeminiSession.swift Sources/GeminiSessionRefresh.swift \
  Sources/GeminiSessions.swift Sources/GeminiNetworking.swift \
  Tests/LiveModelTests.swift -framework CryptoKit -o /tmp/gemini-live-model-tests
/tmp/gemini-live-model-tests gemini-4-argon-low
/tmp/gemini-live-model-tests gemini-4-argon --stream
```

## 致谢

Gemini 网页协议逆向来自 [Sophomoresty/gemini-web2api](https://github.com/Sophomoresty/gemini-web2api)。Anthropic Messages 与工具事件映射参考了 [gemini-for-claude-code](https://github.com/coffeegrind123/gemini-for-claude-code)、[UniClaudeProxy](https://github.com/vibheksoni/UniClaudeProxy) 和 [Anthropic 官方流式协议](https://platform.claude.com/docs/en/build-with-claude/streaming)。

MIT
