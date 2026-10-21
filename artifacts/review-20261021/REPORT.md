# Gemini Free 代码审查与提交验证

日期：2026-10-21。范围：本地尚未提交的 Go + MyGo 桌面重构、兼容 API、
会话与凭据存储、GUI、构建及验证脚本、使用教程。

## 审查结论

发现并修复以下 3 项问题。修复后没有发现剩余的阻塞性代码问题。
这不代表全部在线能力通过验收：已有上游超时和账号能力验证缺口继续保留。
本轮未重新请求真实上游，未将历史失败改写为成功。

## 已修复发现

### P2：监听异常后状态仍为运行中，无法重新启动

`internal/app/service.go` 原先只记录 `Serve` 错误，没有释放当前服务引用。
监听异常后 GUI 仍显示运行，重新启动会被“已经运行”拒绝。

现在仅在失败监听仍是当前实例时清除引用、标记停止并保存错误。
旧监听的迟到错误不会覆盖替代服务。成功重启会清除旧错误。

回归：`internal/app/service_failure_test.go`。
新增测试先复现失败，再验证异常恢复和替代服务的状态不被覆盖。

### P2：删除账号时配置写盘失败会丢失凭据

`internal/app/accounts.go` 原先先删除凭据、再保存配置。
配置保存失败时，账号仍在列表中，但其凭据已被删除。

现在先持久化新配置，再删除凭据；删除凭据失败时恢复原配置，
删除和恢复错误均明确返回。账号列表使用独立副本，成功后再更新内存。

回归：`internal/app/account_deletion_test.go`。
覆盖配置保存失败、凭据删除失败及正常删除；配置保存失败先复现了凭据丢失。

### P2：桌面初始化与界面回调跨线程访问状态

`desktop/desktop.go` 原先从 `WhenReady` 再启动 goroutine 初始化桌面对象，
窗口、控制器等字段与激活、退出回调之间缺少统一的线程归属。

现在在 MyGo 的就绪主线程回调中初始化，通过 `RunOnMain` 展示和聚焦窗口。
本地 MyGo v0.6.0 实现已核对。此项来自代码审查，
没有宣称以桌面 race 测试复现；后端 race 测试不覆盖原生 GUI 回调。

## 最终验证

| 检查 | 结果与边界 |
| --- | --- |
| 后端测试 | 56 个 Test 函数所在测试集通过，启用 race，全进程硬超时 60 秒 |
| 覆盖率 | 最后一次完整运行全项目语句覆盖率 66.4%，app 包 67.1%；不是 GUI 覆盖率 |
| 静态检查 | `go vet ./...`、JavaScript 语法检查、`git diff --check` 通过 |
| macOS | arm64 / amd64 Universal 应用和 DMG 构建成功；严格签名校验通过 |
| Windows | amd64 EXE 和安装器构建成功，最新 EXE 的原生窗口存在且响应 |
| Windows 状态 | anonymous、14 个模型配置、0 账号、API 按测试参数保持停止 |
| 测试清理 | Windows 独立测试进程已结束，私有 control 文件已移除 |
| 文档 | 教程截图与在线记录来自审查修复前实测，成功路径和界面未被本轮修复改变 |

本轮新增 5 个回归测试。两个后端问题有失败测试复现；桌面线程问题以实现核对、
双端构建和 Windows 原生启动检查验证，不冒充完整 GUI 回归。
macOS 使用 ad-hoc 签名，尚未公证；Windows 尚未配置发行者签名。

可复现命令：

```bash
perl -e 'alarm shift; exec @ARGV or die "Cannot execute test: $!\n"' 60 \
  go test -count=1 -race -timeout 60s \
  -coverprofile=/tmp/gemini-free-review-coverage-final.out ./...
go vet ./...
bash scripts/build-desktop.sh
codesign --verify --deep --strict 'dist/darwin-universal/Gemini Free.app'
node scripts/windows-smoke.mjs
```

构建时显式使用本机已配置代理；没有改动项目默认网络配置。
Windows 检查依赖本机 Parallels 和项目共享目录，结果不能替代安装器安装测试。

## 保留的在线失败与未验证项

此前完整接口检查为 18/19 通过；OpenAI 流式请求两次触发 40 秒上游超时，
绕过兼容 API 的直接协议探测也超时，网络、代理或 Gemini 上游的根因尚未确定。
未延长验证超时、自动重试、切换模型或返回假响应。

登录账号的高级模型权限、绘图、长期 Cookie 续期未通过验证。
此前出现的会话 401 和思考档位不符没有被本轮修复证明解决。
托盘右键、关闭窗口隐藏、Windows 全部 GUI 操作及安装卸载也未完整覆盖。

历史真实结果见 [自动化测试报告](../automation-20261021/REPORT.md)。

## 本次构建 SHA-256

| 产物 | SHA-256 |
| --- | --- |
| `dist/darwin-universal/Gemini Free 0.1.0.dmg` | `856730cde08662c16a1584855e574f5b46258bcb85202af48759f8114c19e0fb` |
| `dist/windows-amd64/Gemini Free.exe` | `b116ceca95e16180ab1e4ac0d55088b5894e8e268c24e851876a50695b0180e8` |
| `dist/windows-amd64/Gemini Free Setup 0.1.0 amd64.exe` | `848893b5389825633bbf63fb519e69158085377363a83d038780d2c1f44d3f4e` |

## 提交边界

仅纳入实现、测试、教程、独立匿名测试截图和已检查的文本报告。
真实账号、人设与历史取证、原始 Cookie、管理 token、构建产物、
工具运行目录及无关本地文件不纳入本次提交。
