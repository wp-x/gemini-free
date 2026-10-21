# Gemini Free 自动化测试报告

日期：2026-10-21。对象：Go + MyGo 桌面版，macOS Universal / Windows amd64。

这是代码审查修复前的自动化操作记录；截图和在线失败均保留原始结果。
最新修复、回归及产物哈希见 [代码审查报告](../review-20261021/REPORT.md)。

## 结论

主要 GUI 操作、匿名真实问答、Windows 启动和 Windows → Mac 局域网请求通过。
发现并修复了 1 项请求准入问题。最终接口检查 19 项中通过 18 项；
OpenAI 流式问答出现 40 秒上游超时，单独重测仍失败，不能判定整体完全无问题。
登录账号的高级模型权限未通过本轮验证。

[图文使用教程](../../docs/tutorial/index.html)

## 已修复问题

匿名模式下，GUI 会提示高级模型需要登录，但兼容 API 原先未提前执行该检查，
可能发出上游请求并最终超时。现在在生成引擎创建客户端前统一校验：

```text
此模型需要登录会话，当前为匿名模式
```

回归覆盖 6 个账号模型，确认匿名请求不创建上游客户端。
OpenAI / Anthropic 非流式接口返回带该消息的 502；管理问答以 SSE error 返回同一消息。
新增测试先失败复现，再修复并通过完整回归。

源码：

- `internal/gemini/engine.go`
- `internal/gemini/engine_test.go`
- `internal/app/model_access_test.go`

## 验证矩阵

| 范围 | 结果与边界 |
| --- | --- |
| 后端回归 | 51 个 Test 函数所在测试集全部通过，启用 race 检查；全进程硬超时 60 秒 |
| 静态检查 | `go vet ./...` 通过 |
| 覆盖率 | 全项目语句 65.2%，不等于 GUI 覆盖率；原生桌面入口未被单元测试覆盖 |
| GUI 自动操作 | 26 条成功记录；另保留 1 条测试工具断言错误，修正断言后对应检查通过 |
| 原生 macOS | 中文问题真实返回 42，actual mode 6 / thinking 1；退出后进程结束、重启配置保留 |
| OpenAI 普通问答 | 最终包真实回答 13，实际路由 6 |
| OpenAI 流式问答 | 最终接口检查超时；单独重测再次超时，未宣称通过 |
| Anthropic 普通 / 流式 | 最终包均真实回答 13；流式约 39.2 秒完成 |
| 局域网 | Windows 11 请求 Mac en0：health、14 个模型配置、真实匿名回答 42 |
| Windows 原生启动 | 最终 EXE 进程响应、原生窗口存在；匿名、14 模型、0 账号、按要求未启动 API |
| macOS 包 | Universal 构建成功，`codesign --verify --deep --strict` 通过；ad-hoc 签名，未公证 |
| 响应式 | 375px 无整页横向溢出；模型表内部滚动；设置宽 355px、圆角 12px、滚动条不常显 |

GUI 记录包括启停服务、运行时锁定端口和访问范围、草稿保留、Escape 焦点恢复、
默认模型保存和重载、空问题禁用、生成取消、无账号切换错误、无效 Cookie 错误、
密钥替换与移除冲突、空密钥保存、复制地址、日志搜索与导出。

GUI 记录里的 `duration_ms` 是断言执行时间，不代表整个操作或模型生成耗时。
有一条跨执行环境对象深比较触发 `harness_error`，并非产品错误；
逐字段断言重测通过，原记录保留。

## 尚未解决的真实失败

最终完整接口检查：

```text
18 passed / 1 failed
OpenAI stream: 40.010s，未收到正文
应用日志: read upstream stream: context deadline exceeded
```

显式单独重测：

```text
0 passed / 1 failed
upstream_error: read upstream stream: context deadline exceeded
```

随后用 `gemini-probe` 绕过兼容 API、保留同一代理和 40 秒超时，
同样在向 Gemini 上游请求时超时：

```text
upstream request: ... context deadline exceeded
```

这证明问题也能出现在兼容 API 之外；尚不能确定根因是网络、代理还是 Gemini 上游。
没有通过延长超时、静默重试、切换模型或伪造响应让检查变绿。
程序默认超时 180 秒，本轮独立验证配置为 40 秒；结果仅说明该配置下的实测表现，
不能据此认定默认超时下必定成功或必定失败。

首次完整检查失败消息只显示正文断言不匹配；日志已确认超时。
测试脚本现在优先报告 SSE error，保留原失败记录并另存重测结果。
较早的 `api-results.json` 属于修复前记录，其账号模型检查只断言 502，
不足以证明及时拒绝；最终结果以 `api-results-final.json` 为准。

## 未覆盖范围

- 有效账号登录后的高级模型真实权限、绘图能力、长期 Cookie 续期。
- Windows 完整 GUI 交互、各分辨率视觉、安装器安装 / 卸载全过程。
- 关闭原生窗口后的隐藏行为、菜单栏 / 托盘右键完整操作。
- 长时间稳定性、并发负载，以及全部 14 个模型配置的逐个真实请求。

此前账号验证出现 `session HTTP 401` 和高级思考档位不匹配；
本轮空账号配置不能证明这些问题已解决。14 是配置数量，不是匿名可用模型数量。

## 可复现检查

后端测试带全进程 60 秒硬超时：

```bash
perl -e 'alarm shift; exec @ARGV or die "Cannot execute test: $!\n"' 60 \
  go test -race -timeout 60s \
  -coverprofile=/tmp/gemini-free-automation-coverage.out ./...
go vet ./...
node scripts/windows-smoke.mjs
```

接口脚本需使用独立匿名、零密钥且服务已启动的配置：

```bash
node scripts/api-smoke.mjs <private-control-file> <report.json>
node scripts/api-smoke.mjs <private-control-file> <retry-report.json> \
  'real anonymous OpenAI stream'
node scripts/lan-smoke.mjs <private-control-file> en0
```

管理凭证不写入本报告。不要把私有 control 文件作为公开附件。

## 证据

- [最终完整接口检查](api-results-final.json)
- [单独流式重测](api-stream-retest.json)
- [GUI 检查记录](gui-results.json)
- [原生桌面问答截图](03-question-native.jpg)
- [真实错误日志截图](07-logs-errors.jpg)
- [局域网设置](04-lan-settings.jpg) / [网卡地址](04-lan-addresses.jpg)

截图为实际程序界面，没有使用假回复或测试账号成功状态。
本轮截图中的账号 Cookie 输入为空；未改动用户原有 Swift 配置。

## 自动化操作时的构建产物（审查修复前）

| 产物 | SHA-256 |
| --- | --- |
| `dist/darwin-universal/Gemini Free 0.1.0.dmg` | `4dae696acd2f48342d1e4a72d4310d6cd4199c59ffb5cf18dfbc88817a311acd` |
| `dist/windows-amd64/Gemini Free.exe` | `6b55591d2e720b49409966bdcffcbfd2251b0c0b9450eb6d427eda13ed8868e4` |
| `dist/windows-amd64/Gemini Free Setup 0.1.0 amd64.exe` | `7cf7d5236fc4bde39a57358f783cbe0d4442b278ea5f4551762fe49480e3e83d` |

测试结束：独立 Go 测试服务停止、局域网关闭、匿名模式、0 账号、0 API 密钥。
测试应用保留，用户原有 Swift 进程未停止。
