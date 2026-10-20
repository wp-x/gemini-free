<p align="center">
  <img src="logo.png" width="200" alt="Gemini Free">
</p>

<h1 align="center">Gemini Free</h1>

macOS 菜单栏应用，把 Gemini 网页端转成本地 OpenAI / Anthropic Messages 兼容 API。纯 Swift，内存占用约 14MB。

基于 [gemini-web2api](https://github.com/Sophomoresty/gemini-web2api) 的协议逆向逻辑重写。

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

Gemini 4 Argon：`gemini-4-argon`（默认高档）、`gemini-4-argon-low`、`gemini-4-argon-medium` 与 `gemini-4-argon-high` 已通过真实 API 测试。需配置具有 Argon 权限的登录 Cookie，以及同一网页会话的 `xsrf_token`（`SNlM0e`）和 `gemini_bl`（`cfb2h`）。客户端同时支持 OpenAI 与 Anthropic 接口。应用会发送独立的 Argon 模型路由请求头，并核对上游实际返回的模型与思考等级；权限不足、会话失效导致降级时明确报错，不会把 Flash 回复当成 Argon。

截至 2026-10-20，低、中、高三档均已核对上游响应元数据。过期或混用的会话参数可能使上游返回低档，遇到等级不匹配错误时需同步更新 Cookie、`xsrf_token` 与 `gemini_bl`。

Argon 思考等级可用别名选择，也可使用 `@think=1`（低）、`@think=4`（中）、`@think=2`（高），例如 `gemini-4-argon@think=4`。这是网页端的新思考等级枚举，与旧模型的深度参数不同。

免登录：`gemini-3.8-flash` `gemini-3.7-flash` `gemini-3.6-flash` `gemini-3.5-flash` `gemini-3.5-flash-thinking` `gemini-3.5-flash-thinking-lite` `gemini-auto` `gemini-flash-lite`

要 Gemini Advanced 付费 cookie：`gemini-3.1-pro` `gemini-3.1-pro-enhanced`（没 cookie 也能调，静默回退 Flash）

旧模型保留 `@think=N` 兼容参数，沿用原有编码；Argon 的思考等级使用独立字段。

## 设置

点菜单栏图标 → 设置，可改端口、局域网开关、API Key 鉴权、Cookie 文件、代理、默认模型。配置存 `~/.config/gemini-web2api/config.json`，格式和原项目通用。

- **局域网访问**：默认开启（监听 0.0.0.0），同网段设备可用本机 IP 调用；关闭则仅本机（127.0.0.1）。
- **API Key 鉴权**：填一个或多个 Key（逗号分隔），客户端需带 `Authorization: Bearer <key>`；留空则免密。开放局域网时建议设置。

代理填 `http://127.0.0.1:7890` 这类 HTTP 代理即可。

## 编译

```bash
./build.sh
```

运行回归测试：`bash test.sh`，每个测试进程限时 60 秒。已配置本机 Cookie 后，可手动测试真实 Argon 路由：

```bash
swiftc Sources/Config.swift Sources/Engine.swift Sources/GeminiPayload.swift \
  Sources/GeminiResponse.swift Sources/Models.swift Sources/Util.swift \
  Tests/LiveModelTests.swift -framework CryptoKit -o /tmp/gemini-live-model-tests
/tmp/gemini-live-model-tests gemini-4-argon-low
/tmp/gemini-live-model-tests gemini-4-argon --stream
```

## 致谢

Gemini 网页协议逆向来自 [Sophomoresty/gemini-web2api](https://github.com/Sophomoresty/gemini-web2api)。Anthropic Messages 与工具事件映射参考了 [gemini-for-claude-code](https://github.com/coffeegrind123/gemini-for-claude-code)、[UniClaudeProxy](https://github.com/vibheksoni/UniClaudeProxy) 和 [Anthropic 官方流式协议](https://platform.claude.com/docs/en/build-with-claude/streaming)。

MIT
