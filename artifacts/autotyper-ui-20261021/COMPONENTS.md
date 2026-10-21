# AutoTyper / AutoGLM 设计组件调查

调查日期：2026-10-21。本版以用户已经登录的 AutoGLM 主界面为主要参考，
替代此前仅根据登录页提出的通用管理后台布局。下文分别标记运行观察、
静态恢复、视觉估算、项目适配和未知项，不宣称已提取全部按钮状态。

本文随项目保存；所列参考截图、原始取证和辅助解析文件仅保留在本地，不随版本分发。
其中登录后截图含真实账号、人设和历史，不纳入仓库。

## 两个版本，分别取证

| 来源 | 身份 | 用途 |
| --- | --- | --- |
| 当前安装应用 | `/Applications/AutoGLM.app`，1.7.0（128） | 登录后的布局、按钮外观及有限交互；本次设计优先依据 |
| 用户指定 DMG | `/Users/mac/Downloads/AutoTyper_091001.dmg` 内 `/Volumes/AutoGLM/AutoGLM.app`，1.19.0（150） | Rea 资产、Mach-O、Swift 类型、导航样式常量；另有此前登录页运行记录 |

1.7 可执行文件 SHA-256：
`550ac4ec864969f4d630bd194c635215ff7e7ffd9e53b8f2a8dcf77f28e5b58d`。
1.19 可执行文件 SHA-256：
`fa74975568ae2967781fea04b1b6f557713ae3a90001a205ca3778d138fd2dd6`。
两者 Bundle ID 均为 `com.zhipu.agent`。1.7 的关于页同时显示 Xiaoao 名称。

**不能用 1.19 的静态数字证明 1.7 的精确实现。**
1.7 截图决定本次结构和视觉方向，1.19 恢复值作为有来源的组件适配候选。

## 登录后运行证据

以下为主代理通过官方 CUA 保存的 1.7 窗口截图，原图 2160×1600 px，
对应本次约 1080×800 pt 窗口。没有为调查创建、删除或改写用户数据。

| 截图 | 可直接观察的范围 |
| --- | --- |
| 首页 `logged-in/home.png` | 侧栏、四统计块、当前人设入口、按日期分组的历史 |
| 历史更多菜单 `logged-in/history-menu.png` | 更多按钮展开后的白色小面板及两个命令 |
| 账号设置 `logged-in/account.png` | 设置覆盖层、导航、标识值与复制图标、退出操作外观 |
| 通用设置 `logged-in/general.png` | 分组设置行、快捷键控件、灰色开关 |
| 关于 `logged-in/about.png` | 版本胶囊、检查更新按钮、外链行、信息清单 |
| 词典 `logged-in/dictionary.png` | 新增按钮、筛选控件、列表、尾部箭头 |
| 人设 `logged-in/personas.png` | 当前摘要、列表与详情分栏、选中态、禁用输入和图标 |
| 首页 AX `logged-in/home-ax.txt` | 更多按钮 `ellipsis`、当前人设按钮、统计和列表的可访问性结构 |

截图含真实账号标识、人设和历史，属于分析证据；不把其中内容填入 Gemini
Free 产品、演示数据或默认配置。截图左上共享捕获标识、指针光晕不属于应用组件。

### 真实整体结构

| 项目 | 当前观察 | 证据性质 |
| --- | --- | --- |
| 窗口框架 | 内容延伸到顶部；没有单独占一横条的白色标题栏 | 运行像素 |
| 主侧栏 | 约 208 pt 宽，浅灰底；顶部品牌，主导航居上，设置和反馈靠底 | 视觉估算 |
| 品牌 | 约 36 pt 图形，右侧两行中文名与英文名 | 视觉估算 |
| 主区域 | 白底，左侧约 24 pt 内边距，页面独立滚动 | 视觉估算与可见滚动条 |
| 导航选择 | 图标与文字同排，浅灰选中背景，颜色不靠品牌橙区分 | 运行像素 |
| 首页上半部 | 一行问候与当前人设入口，下一行四个浅灰统计块 | 运行像素 |
| 首页下半部 | 留白后出现日期标题；同日历史用细边界和分隔线组织 | 运行像素 |
| 设置 | 在原页面上覆盖约 840×660 pt 面板，背后首页变暗 | 视觉估算 |
| 设置内部 | 左侧约 200 pt 冷灰导航，右侧浅色正文，右上圆形关闭 | 视觉估算 |
| 设置选中 | 左侧条目使用白色背景；与主侧栏的浅灰选中不同 | 运行像素 |

上述尺寸为当前窗口状态，未验证最小窗口、缩放、深色主题或 Windows。
设置面板和人设详情确实有明显大圆角，不宜由此把每个普通控件都设为同一圆角。

## 已观察组件及交互粒度

| 组件 | 原版构成与外观 | 已观察／未观察边界 |
| --- | --- | --- |
| `BrandHeader` | 图形与两行品牌字；与主导航有明显间隔 | 外观已观察，点击行为未知 |
| `SidebarPrimaryItem` | 线性图标、文字、浅灰选中面 | 首页、词典、人设页面已观察；运行悬停态未测 |
| `SidebarSecondaryItem` | 底部设置、意见反馈，图标与文字同排 | 设置已打开；未发送反馈 |
| `StatTile` | 浅灰底、上方小标签、较大数值、局部插图 | 四块均可见；不能从图片证明可点击 |
| `CurrentPersonaLink` | 首页右上浅灰前缀、当前值与铅笔图标 | 点击进入人设页；不是已证实的浮动选择菜单 |
| `HistoryGroup` | 日期标题、右侧更多；正文与小号时间两层 | 默认显示已观察；未执行历史写入或删除 |
| `MoreMenu` | 白色小面板、轻阴影、灰色图标与普通灰色文字 | 含清空历史、批量删除；再次点更多收起；本次 Esc 未收起 |
| `SettingsOverlay` | 遮罩、大面板、内部分栏、右上圆形 X | 账号、通用、关于内容已观察；拖动及缩放未知 |
| `SettingsNavItem` | 图标、文字、白色选中底 | 三项切换已观察；运行悬停态未测 |
| `SettingSection` | 小图标与粗体组标题，下方以留白分组 | 键盘快捷键、App 行为可见；不是每行一个卡片 |
| `SettingRow` | 左标题及灰色说明，右侧紧凑控件 | 开关显示为灰色轨道、白色滑块；未为了取证切换系统设置 |
| `HotkeyControl` | 浅灰圆角键位区域，右端细分隔与清除 X | 仅外观；录入、冲突检测和清除未知 |
| `CopyIconButton` | 小号浅灰圆底复制图标，标识值在左 | 账号页面可见；未复制真实账号信息 |
| `DangerAction` | 注销为红色描边；退出为红色实心胶囊 | 仅外观；未注销、退出或进入确认流程 |
| `VersionBadge` | 灰色胶囊承载版本号 | 1.7.0（128）可见 |
| `OutlineAction` | 检查更新使用刷新图标、描边浅色背景 | 未触发更新检查 |
| `ExternalLinkRow` | 左侧标题，最右上箭头 | 法律信息行可见；未打开外链 |
| `FilterChoice` | 词典页三个带图标胶囊；选中项浅暖色描边 | 默认所有项可见；筛选请求及切换结果未覆盖 |
| `AddAction` | 词典右上加号与新增热词，橙色实心 | AX 点击一次未观察到弹窗；不能标记为已验证新增 |
| `DictionaryRow` | 左图标和词条，右侧 chevron，白底细边 | 列表已观察；编辑、保存、失败状态未知 |
| `PersonaSummary` | 顶部当前人设摘要、图形、右侧新增操作 | 默认摘要已观察；新增流程未执行 |
| `SelectablePersonaRow` | 图标、名称、快捷键小标、来源说明 | 已选行为浅珊瑚底、细边、尾部圆勾；未写入人设 |
| `PersonaDetail` | 左列表与右详情分栏，名称、图标网格、描述 | 内置项名称与图标禁用；复制并修改按钮可见但未执行 |

历史菜单的删除命令在本次截图中是普通灰色，不能因命令含义自行声称原版使用红色。
组件表覆盖当前主界面中的可见控件；没有运行实测全部按下、悬停、加载、错误状态。

## 1.19 静态恢复的导航参数

Rea 分发元数据见本地 `native-ui-dispatch.json`，Evidence
`ev_d296dd89504a6012e95f952260bb8d1663eefd05785723b15a1fd3e2ed65b7cb`。
Swift 通用元数据覆盖不完整，`swift_decls` 为空不等于没有 SwiftUI。
后续用文件解析器读取类型及字段描述符：
`inspect-swift-ui.mjs`、
`swift-ui-fields.json`。这一步不执行应用代码。

恢复到 `SidebarNavMetrics` 的存储字段顺序：
`rowHeight, cornerRadius, horizontalPadding, iconSize, iconTextSpacing,
fontSize, fontWeight, textColor, kerning`，类型描述符为 `0x100654558`。
`SidebarNavButton` 有 `isActive/isCollapsed/style/hoverHighlightEnabled/_isHovering`，
并非只凭图片猜测存在这些状态。

结合本地导航指令 `sidebar-metrics-arm64.txt` 的值装配：

| 参数 | style 分支 0，推定 primary | style 分支 1，推定 secondary |
| --- | --- | --- |
| 行高 | 36 pt | 34 pt |
| 圆角 | 10 pt | 12 pt |
| 横向 padding | 14 pt | 14 pt |
| 图标 | 20 pt | 20 pt |
| 图标文字间距 | 10 pt | 8 pt |
| 字号 | 14 pt | 14 pt |
| 字重 | SwiftUI medium | SwiftUI semibold |
| 文字 sRGB | `#292929` | `#525252` |
| kerning | -0.14 pt | 0 pt |

数值、SwiftUI 字重调用、字段顺序是静态证据。primary/secondary 与 0/1 的对应
依据 `SidebarNavStyle` 无关联值枚举字段顺序推断，未恢复原 Swift 源码。
不把此处 secondary 数值冒充 1.7 设置覆盖层导航的实现。

本地颜色与背景分支指令 `sidebar-color-flow-arm64.txt` 进一步证明：

| 状态／颜色 | 恢复结果 | 地址链 |
| --- | --- | --- |
| selected | sRGB `#EBEBEB`，opacity 1 | isActive 分支 `0x10044d9e4` → initializer `0x100317024` → double `0x1006147e8` |
| hover | sRGB `#F0F0F0`，opacity 1 | hoverHighlightEnabled 且 State 为真 → `0x100317040` → immediate `0x3fee1e1e1e1e1e1e` |
| 普通 | `SwiftUI.Color.clear` | `0x10044da68` |
| primary text | sRGB `#292929` | `0x100317060` → double `0x100614800` |
| secondary text | sRGB `#525252` | `0x10031707c` → double `0x100614cc8` |

颜色 initializer 把相同 double 放入 RGB，经 `0x100169bb0` 进入显式 sRGB
Color 初始化；`0x100169c24` 设置 opacity 1。选中优先于悬停；非选中、
未开启 hoverHighlightEnabled 或未悬停时为透明。圆角使用 `.continuous`；
按钮使用 `PlainButtonStyle`。这些为 1.19 静态分支，未在 1.7 上实测 hover。

完整数值与字节位置保存于本地 `sidebar-style.json`。
ARM64 slice 文件偏移为 9388032；上述常量均在 `__TEXT`，
对应绝对文件偏移为 `sliceOffset + vmAddress - 0x100000000`。
llvm-objdump 全扫描报少数 invalid-instruction 警告；此处引用范围均有具体有效指令，
不声称完整反汇编或已完成深度反编译。Ghidra/Hopper/IDA 当前不可用，未额外安装。

## 截图颜色不能等同代码常量

1.7 截图嵌入 `LG ULTRAWIDE` 显示器 ICC 配置。用 AppKit 读取
`home.png` 中 (300,235) 的主导航选中底，原始通道是 `232,232,232`；
转换到 sRGB 后约为 `237,237,237`。侧栏 (150,750) 原始通道
`243,243,243`，转到 sRGB 约为 `245,245,245`。

因此不能直接把截图某个原始灰阶命名为 1.7 源码中的 `#EAEAEA`，
也不能用这些像素否定另一版本中恢复的 `#EBEBEB`。
本地 `screenshot-color-samples.json` 保留取样方法与来源。
白色主内容面、浅灰侧栏、浅灰选中是明确的视觉关系；精确 CSS 色值属于适配决策。

## 资源与组件实证

- `Contents/Resources/sidebar_icon_{home,persona,dictionary,points,settings,feedback}.png`
  是独立资源，不在 Assets.car 清单内。抽查 home 为 80×80 px；
  1.19 导航渲染图标大小由上述指令恢复为 20 pt，资源尺寸本身不能推出命中区域。
- `LocalizedImages.bundle/zh-Hans.lproj/sidebar_logo.png` 为 508×144 px；
  实际侧栏品牌组合以运行截图为准。不复制原版品牌图形或字样。
- 本地 `assets.json` 的 93 条记录含目录、多尺寸 AppIcon 和打包项；
  不能称为 93 个按钮。此清单没有单独 Color 记录，不能声称从 Assets.car 提取了全局颜色。
- 本地 `ui-localization-zh.json` 保留结构化中文文案：
  首页、词典、人设、积分、设置、意见反馈；四统计；账号、通用、关于。
  本地化键存在不等于当前 1.7 页面对所有入口都显示，例如积分仅有静态证据。
- 打包 `README-Regression.md` 提及内置人设复制编辑、删除确认、快捷切换等
  预期回归要求。这是产品测试文档，不是上述流程本次执行成功的证据。
- `HomeStatCardModel` 静态字段含 `title/value/imageName/imageSize/imageOffset`；
  `SidebarView` 包含当前路由、先前主路由、折叠、设置覆盖层与帮助状态。
  这些结构佐证模块边界，不能单独证明具体动画、宽度或主题。

## Gemini Free 组件适配

本节是项目设计决策，不冒充 AutoGLM 原版功能或源码。

| 项目组件 | 保留的参考结构 | Gemini Free 内容与适配 |
| --- | --- | --- |
| `AppShell` | 左浅灰导航、右白正文、顶部内容贯通 | 约 208 px 侧栏与 24 px 内容边距；不增加独立顶部工具栏 |
| `BrandHeader` | 约 36 px 图形与两行品牌名 | 项目自己的 Gemini Free 名称和图形 |
| `SidebarItem` | 图标、文字、浅灰选中，主次分组 | 首页、问答、模型为主导航；设置和运行日志靠底 |
| `OverviewStats` | 四个同排浅灰统计项，小标签与大数值 | 运行状态、模型配置数、登录会话数、API 密钥数，均来自真实状态 |
| `ActivityList` | 日期分组、正文加时间、细边界列表 | 最近运行日志与完整日志入口；当前不提供持久问答历史或原版删除菜单 |
| `SettingsOverlay` | 页面上覆盖的分栏设置，右上关闭 | 登录会话、通用连接、访问密钥三类；不是独立后台主页面 |
| `SettingRow` | 左标题与描述、右侧紧凑控件 | 服务端口、运行选项；布尔值用开关，命令用按钮 |
| `SessionList/Form` | 紧凑列表、分组表单、图标操作 | 会话列表与展开新增表单、刷新状态、具名删除确认；不采用人设左右详情布局 |
| `ModeChoice` | 清楚呈现当前选择 | 匿名、登录会话两种模式；无效、校验中、可用状态来自后端 |
| `ModelTable` | 紧凑信息层次与克制分隔线 | 表格分列呈现模型别名、请求路由、访问条件和验证状态；另设默认模型选择 |
| `IconButton/Menu` | 原版图标按钮与小菜单的粒度 | 使用项目 Lucide 图标和 tooltip；图标本身与可点击面积分开设置 |
| `TrayMenu` | 使用平台原生菜单的交互习惯 | macOS 状态栏、Windows 托盘；与 GUI 共享真实状态 |

导航数值可采用上表作为明确注明来源的 1.19 适配值；界面 `letter-spacing`
使用 0，不照搬原版负 kerning。普通控件不统一套用登录页 40 pt 高和大圆角。
复制、校验、启动、停止等命令需显示真实执行结果；失败保留明确错误。
管理工具不必复制词典的暖色说明区、人设摘要插画及语音快捷键业务。

### 按钮视觉复核，2026-10-21

登录后的 1.7 关于页使用浅底细描边的检查更新按钮，词典新增使用橙色实心，
账号退出使用红色实心。不能把原版概括为全部浅灰按钮，也没有证据说明
所有普通命令都应使用黑底。当前项目原有的黑底启动、保存与周围浅灰控件
反差过强，改为 `#EEEEEE` 底、`#E1E1E1` 细边和深色文字。
悬停 `#E5E5E5`、按下 `#DCDCDC` 是项目适配值；保留 40px 常规高度、
8px 圆角、键盘焦点和红色危险确认。次级操作使用白底细边，
工具图标在交互时出现浅灰底，禁用按钮不产生悬停和按下填充。

## 旧登录页证据保留

此前分析是 DMG 对应 1.19 的登录前窗口，与本次 1.7 已登录页面分别归档。
当时运行程序映像 SHA-256 与 DMG 内一致；AppTranslocation 路径不改变身份。
Rea 运行 Evidence：
`ev_45e7bc20a662e797a53e1d2f329b354b6990af88e1f365a19e3e3c06ec41234e`，
见本地 `native-login-runtime.json`、`rea-login.png`、`login-default.png`、
`login-focused.png`、`login-code-focused.png`、`login-ax.txt`。

已验证手机号可聚焦、Tab 到验证码、空表单的发送验证码与登录均禁用；
未填手机号、发送验证码或执行登录。白底居中表单不代表登录后应用框架，
不将此登录墙移植为支持匿名模式的 Gemini Free 首屏。
当时观察的 macOS 应用／显示／窗口菜单不等于状态栏托盘菜单。

## 覆盖与剩余未知

| 问题 | 状态 | 依据及限制 |
| --- | --- | --- |
| 两个目标版本身份 | 已回答 | plist、SHA-256、1.7 关于页 |
| 登录后结构与主要页面外观 | 已回答 | 七张当前 1.7 运行截图 |
| 导航、设置切换、首页人设入口、历史更多 | 部分回答 | 有有限交互记录，未覆盖全部键盘操作 |
| 导航数值、背景分支 | 已回答，仅 1.19 | 字段元数据、ARM64 指令与 double 字节 |
| 可见按钮与默认／选中／部分禁用外观 | 部分回答 | 本文组件表；未冒充全部状态 |
| 新增热词、保存、人设复制、删除、注销、反馈 | 未解决 | 未执行写入；新增 AX 一次未见弹窗 |
| 全部 hover、按下、加载、错误、动画 | 未解决 | 静态导航 hover 分支不等于运行全覆盖 |
| 托盘右键菜单、关闭窗口生命周期 | 未解决 | 原版状态栏菜单未验证 |
| 1.7 精确字体／源码参数、深色与缩放 | 未解决 | 视觉估算和有 ICC 的截图不足以证明 |
| 原版 Windows 行为 | 未解决 | 当前取证对象均为 macOS |
