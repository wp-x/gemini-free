# Gemini Free Go Desktop Implementation Plan

**Goal:** 用 Go 和 MyGo 交付 macOS / Windows 桌面客户端，支持匿名与登录会话、管理 GUI、托盘以及兼容 API。

**Architecture:** 生成接口隔离上游协议、兼容 API 与桌面管理层。GUI 和托盘复用同一个应用控制器；账号凭据与会话参数绑定存储，匿名请求独立构造，不回退模式或模型。

**Tech Stack:** Go 1.27.1，MyGo v0.6.0，系统 WebView，嵌入式 HTML/CSS/ES modules，标准库 HTTP。

---

用户已授权实施，本计划直接执行。保留现有 Swift 工程作为迁移参考，不覆盖用户配置及未提交改动。手工代码修改使用 apply_patch。

### Task 1: 上游与会话

Files: `internal/generation/`, `internal/gemini/`, `internal/config/`

1. 建立生成接口、模型定义、102 字段请求编码与响应帧解析。
2. 添加匿名隔离、路由校验、畸形响应、取消与增量输出测试。
3. 移植 Cookie 导入、轮换、同账号参数同步；通过依赖注入存储凭据。
4. Run: `go test -timeout 60s ./internal/generation ./internal/gemini ./internal/config`。
5. Run: 真实匿名普通与流式问答，记录请求及实际路由。

### Task 2: 兼容 API

Files: `internal/api/`

1. 迁移 `/v1/models`、`/v1/chat/completions`、`/v1/messages`。
2. 验证普通响应、SSE、工具选择、工具返回、多轮文本与错误传播。
3. Run: `go test -timeout 60s ./internal/api`，预期通过。

### Task 3: 组件与 GUI

Files: `desktop/web/`, `DESIGN.md`, `artifacts/autotyper-ui-20261021/COMPONENTS.md`

1. 复用 Rea 制品分析，补实际可见页面及控件状态证据。
2. 写清已观察设计、产品适配和未知项。
3. 实现服务概览、问答、会话、模型、连接和日志视图。
4. 渲染检查桌面及窄窗口，验证真实服务数据、表单错误及请求取消。

### Task 4: 桌面控制与生命周期

Files: `internal/app/`, `desktop/`, `cmd/gemini-free/`

1. 配置持久化、凭据存储、服务启停与日志使用同一个控制器。
2. 管理端仅监听 loopback，使用随机令牌鉴权；令牌不写日志。
3. 用 MyGo 创建窗口与托盘；窗口关闭隐藏，退出关闭服务。
4. 登录会话导入、续期、删除和权限探测都真实执行并显示错误。

### Task 5: 构建与验收

Files: `scripts/`, `README.md`

1. Run: `go test -race -timeout 60s ./internal/...` 与 `go vet ./...`。
2. 构建 macOS arm64、Windows amd64；记录无法实机验证的项目。
3. 实际启动 macOS GUI，检查截图、模式切换、托盘和问答。
4. 更新运行、配置、Cookie 和构建说明，保留匿名测试证据。

## Status

- [x] Swift 匿名真实问答、中文计算与流式测试。
- [x] Rea 制品身份和资源初步调查。
- [x] Go 协议和会话；匿名真实问答与增量输出通过，高级模型权限严格验证。
- [x] 兼容 API，普通、流式、工具及错误测试通过。
- [x] GUI 与组件规范，按登录后的 AutoGLM 重新取证并完成第二版。
- [x] MyGo 桌面及托盘，窗口关闭隐藏、退出停止服务。
- [x] 双端构建、macOS 渲染与问答、Windows 原生窗口启动验证；高级账号能力未通过。

### 2026-10-21 登录后视觉复核

用户拒绝第一版表单式 GUI，要求重新分析已登录 AutoGLM。

- [x] 观察首页、词典、人设、账号、通用、关于及历史更多菜单。
- [x] 区分已登录 `/Applications/AutoGLM.app` 1.7.0（128）与 DMG 1.19.0（150）。
- [x] 记录 REA 导航布局及 sRGB 色值，并保存登录后截图。
- [x] 改用 MyGo 隐藏标题栏，保留系统窗口按钮。
- [x] 重做首页、设置面板和共享控件，保持真实功能接线。
- [x] 检查 macOS 实际窗口及 1080、700、375 像素视口，保存改版后证据。
- [x] 重打包双端并用最终 EXE 复测 Windows 原生窗口启动与响应。
- [x] 验证设置分类、表单草稿、Escape 返回、会话失败提示和匿名 GUI 问答 42 / mode 6。

自动化操作证据及测试边界见
[自动化测试报告](../../artifacts/automation-20261021/REPORT.md)，最新产物
SHA-256 见 [代码审查报告](../../artifacts/review-20261021/REPORT.md)。Windows 本轮覆盖启动与管理状态，
未宣称完整视觉交互覆盖。账号会话刷新仍返回 401，高档权限未通过。

### 2026-10-21 标注修正与局域网

- [x] 统一下拉框材质、隐藏滚动条轨道并保留滚动、统一弹窗 12px 圆角。
- [x] 删除品牌副标题与重复模式文字，增加模式图标、服务状态灯。
- [x] 改为圆形发送按钮，验证空输入、生成中停止按钮和真实匿名响应。
- [x] 主界面及连接设置增加明确的“启动服务 / 停止服务”文字按钮并实测。
- [x] 简化会话导入，名称可选、支持文件入口，高级参数默认折叠。
- [x] 增加局域网开关、地址发现和复制，管理端保持本机监听。
- [x] 从 Windows 11 通过 Mac 局域网接口读取模型并完成真实匿名问答，返回 42。
- [x] 后端 race 测试、静态检查、双端打包及 Windows 原生启动验证通过。
