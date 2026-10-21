# Gemini 匿名模式在线测试

日期：2026-10-21。

## 结论与范围决定

当前匿名问答链路可用，Go 重构正式纳入以下两种模式：

1. 匿名模式：不读取或发送登录 Cookie、账号 Authorization、账号编号及登录会话参数。
2. 登录会话模式：使用所选账号的 Cookie 和同一会话参数，按实际权限请求模型并校验返回档位。

目标平台已由用户确认为 macOS 和 Windows。菜单栏或托盘、GUI 和 API 服务共用所选模式。
本轮新增在线测试及文档，没有修改现有业务逻辑；测试时 Go GUI 的模式选择尚未实现。
后续 Go 桌面版的操作和验证见 [自动化测试报告](../automation-20261021/REPORT.md)。

下文的原始响应、日志和测试二进制仅保存在本地，不随版本分发。

## 隔离与复现条件

- 使用新的 `Store()`，不调用 `Store.load()`。
- 通过 `Engine(config:sessions:)` 注入无账号会话，不使用 `Engine.shared`。
- 会话 Cookie 为空，Cookie 文件、SAPISID、账号身份、轮换凭据和 XSRF token 均为空。
- 复用生产代码的请求编码、网络配置及响应解析。
- 生产网络配置为 ephemeral，禁用 Cookie 存储和自动接收。
- 不启动登录自动续期，不读取或修改既有账号配置及 Cookie。
- 显式使用系统已配置的 HTTP 代理 `http://127.0.0.1:7890`。
- 单次请求超时 40 秒，每个在线测试进程由 Perl alarm 强制限制在 60 秒。
- 不进行自动重试，不使用模拟响应。

直连 `gemini.google.com:443` 建立连接超时，没有收到 HTTP 响应。
通过代理访问匿名首页返回 HTTP 200，故改用代理测试；未将直连超时判为匿名模式失效。

## 实测结果

| 测试 | 结果 | 证据 |
|---|---|---|
| 匿名原始请求，`gemini-3.8-flash` 别名 | HTTP 200，正确返回指定标记；请求编号 1，实际编号 6 | `flash-raw.log`、`flash.raw` |
| 生产 Engine 普通问答，`gemini-flash-lite` | 正确返回 `ANONYMOUS_OK_7391`，约 3.28 秒 | `flash-lite-answer.log` |
| 生产 Engine 流式问答，`gemini-flash-lite` | 收到 2 段增量，拼接后标记一致，约 3.19 秒 | `flash-lite-stream.log` |
| 中文计算题：17+25 | HTTP 200，正确返回 `42`，实际路由编号 6 | `chinese-math.log`、`chinese-math.raw` |
| 中文较长流式回答 | 收到 12 段增量，约 6.38 秒；仅验证输出非空和流式传输，不将内容正确性计为自动断言通过 | `chinese-stream.log` |
| 匿名 Argon 高档 | 请求收到 HTTP 200，但返回思考等级 1，与预期 2 不一致，被生产校验拒绝 | `argon-anonymous.log` |
| 匿名 Argon 低档 | 收到 HTTP 200 后读取响应超时，未得到成功结论 | `argon-low-anonymous.log` |

HTTP 200 不等于模型路由或生成成功。高级档位测试没有作为成功计入。
本轮没有验证登录账号权限、全部模型别名、多轮上下文、图片生成或 GUI。

## 对重构的约束

- 匿名可用性已确认，但不能承诺所有模型名称或版本都与本地别名一致。
- 编号 6 在当前项目中映射为 `gemini-flash-lite`；这不单独证明厂商营销名称或具体版本。
- 不把高级档位返回低档包装为成功，不在匿名与登录模式之间自动切换。
- 模型列表应区分已验证能力与尚未验证能力。
- 登录模式仍需独立验证实际账号权限、会话参数及续期。

## 编译与复现

在项目目录编译：

```bash
swiftc Sources/Config.swift Sources/Engine.swift \
  Sources/GeminiPayload.swift Sources/GeminiResponse.swift \
  Sources/Models.swift Sources/Util.swift Sources/GeminiSession.swift \
  Sources/GeminiSessionRefresh.swift Sources/GeminiSessions.swift \
  Sources/GeminiNetworking.swift Tests/AnonymousLiveTests.swift \
  -framework CryptoKit -o artifacts/anonymous-20261021/anonymous-live
```

普通问答：

```bash
env GEMINI_PROBE_PROXY=http://127.0.0.1:7890 \
  perl -e 'alarm shift; exec @ARGV or die "Cannot execute probe: $!\n"' \
  60 artifacts/anonymous-20261021/anonymous-live answer gemini-flash-lite
```

将 `answer` 替换为 `stream` 测试增量输出。
`raw` 会报告上游状态、路由及思考等级；可通过 `GEMINI_PROBE_RESPONSE` 指定原始响应保存位置。
自定义问题使用 `GEMINI_PROBE_PROMPT`，精确预期值使用 `GEMINI_PROBE_EXPECTED`；
未指定自定义预期值时，日志明确标记 `TEXT_RECEIVED validation=nonempty_only`。
