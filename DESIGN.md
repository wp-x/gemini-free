# Gemini Free desktop design

## 1. Visual theme and evidence

Use the observed AutoGLM application structure: a quiet gray sidebar, a white
workspace, compact gray selected navigation, four flat overview metrics, dated
activity lists, and a centered settings dialog with a pale cool sidebar.

The logged-in application reports **1.7.0 (128)** in its About screen. The supplied
DMG contains **1.19.0 (150)**. These are different versions. Logged-in screenshots
under `artifacts/autotyper-ui-20261021/logged-in/` are the visual reference for this
revision; DMG static evidence must not be described as proof of the older running
application's exact implementation.

## 2. Color palette

| Token | Value | Role |
| --- | --- | --- |
| Canvas | `#FFFFFF` | Main content |
| Sidebar | `#F5F5F5` | Main navigation |
| Selected | `#EBEBEB` | Selected main navigation |
| Settings sidebar | `#F5F4F8` | Settings categories |
| Input | `#F3F3F3` | Editable controls |
| Action surface | `#EEEEEE` | Everyday primary commands |
| Action border | `#E1E1E1` | Quiet button boundary |
| Action hover / press | `#E5E5E5` / `#DCDCDC` | Pointer feedback |
| Ink | `#292929` | Main text |
| Secondary | `#858585` | Metadata |
| Divider | `#EDEDED` | Hairline grouping |
| Positive | `#35734E` | Confirmed running state |
| Error | `#B43D35` | Actual errors |

Selected `#EBEBEB`, hover `#F0F0F0`, and ink `#292929` are from DMG 1.19 static
REA evidence. Other colors are migration choices. The 1.7 screenshots embed an
LG ULTRAWIDE ICC profile; raw screenshot RGB is not treated as exact sRGB CSS.

## 3. Typography

Use the system stack: `-apple-system`, `BlinkMacSystemFont`, `SF Pro Text`,
`Segoe UI`, `PingFang SC`, sans-serif. Tracking is zero throughout.

Page title is 24px/600, settings title 29px/600, main navigation 14px/600, body
13px, compact controls 12px, and metadata 10-12px. Chinese reading surfaces use a
1.65-1.85 line height. Numeric status uses tabular figures.

## 4. Components and evidence limits

REA-derived navigation constants: visible row height 36, icon 20, horizontal
padding 14, icon-label gap 10, corner radius 10, and row gap 6. A small extended
hit area preserves at least 40px interaction without changing visible geometry.

Settings dialog size near 840 by 660 and sidebar near 196 are migration values
estimated from the running screenshots. Following user feedback, all dialogs use
a shared 12px radius, including narrow layouts, rather than the earlier 36px
settings radius. The close control is visually
32px with an extended 40px hit area. Form controls and flat metrics use 8px radius.
Rows pair a left label and description with a compact right input or gray switch.
Selects use the same flat gray surface, 40px height and 8px radius as inputs,
with a Lucide chevron instead of WebKit's default gradient and dual arrows.
Their native selection, disabled state, focus, and keyboard behavior remain.

Everyday commands, including Start Service, Save, and Add Session, use a pale
gray fill, dark text, a fine border, and the shared 8px radius. Secondary commands
use a white fill with a lighter border; utility icons stay unfilled until hover
or press. Destructive confirmation retains red emphasis. Disabled buttons do
not acquire hover or press fills, and keyboard focus remains clearly outlined.
The reference's About screen uses a light outlined update action; dictionary
creation uses orange and account exit uses red. The neutral primary treatment
here is a project adaptation, not a recovered reference color.

All icons are vendored Lucide assets. The product uses its own existing Gemini
logo. AutoGLM private branding, account details, and unrelated input-method
features are not copied.

## 5. Layout

Main sidebar is 208px. Brand begins at x18, y38 with a 36px image and a single
product name. Main content begins at sidebar plus 24px and below the 32px titlebar zone.
The upper navigation contains 首页, 问答, 模型; 设置 and 运行日志 are at the bottom.
The shell fills the window without scrolling the HTML document. Main content
scrolls independently; opening any dialog locks background content. Scrollbar
tracks are hidden in WebKit and standard CSS without disabling wheel, touchpad,
or keyboard scrolling. Overflowing model tables are focusable for keyboard use.

Homepage metrics show actual service status, model count, account count, and API
key count. A compact mode bar and compatibility addresses precede activity
grouped by Beijing calendar date. Metrics are an explicit screenshot reference;
ordinary page sections stay unframed.
The sidebar footer is a status light with an accessible name and tooltip.
The header uses an eye-off icon for anonymous mode and a user icon for an account.
An explicit Start Service / Stop Service button stays visible in the main header;
connection settings repeats that command while its modal is open.
The question form uses a fixed 40px circular send control, replaced by a stop
control during generation. Empty questions disable sending.

## 6. Depth and window behavior

Only settings and confirmation dialogs use elevation. Settings backdrop is a
20% black overlay; the rounded dialog has a restrained shadow. Native window
buttons remain native. The frontend drag strip uses `--app-region: drag`, height
`max(32px, var(--mygo-titlebar-height, 0px))`, excluding the left native buttons
and right Windows button area.

## 7. Interaction rules

Settings uses a native HTML dialog: focus containment, Escape handling, and focus
return to its trigger. URLs `view=accounts`, `view=settings`, and `view=keys` open
the corresponding category with the homepage behind it. Browser Back closes a
settings visit. Category changes preserve drafts. Polling only updates data.

New account entry is an inline disclosure. Removing a named account requires a
local confirmation dialog. API failures remain visible. Anonymous and account
modes change only through an explicit apply action. No local storage holds
credentials. Requested model aliases and actual routes remain distinct.
Cookie or exported session JSON is the only required account input. Names are
optional and generated when absent. Account number, XSRF, and session build live
in a collapsed advanced group; imported account numbers are retained. Session
refresh retrieves XSRF and build from the account page.

The connection category exposes an explicit LAN switch. Saved scope controls
the listener, and changing scope requires stopping the service. Discovered
interface addresses are shown with separate OpenAI and Anthropic copy actions.
Wildcard bind addresses are never shown as client URLs. Discovery errors and
offline states remain visible; no address is fabricated. Management stays on
loopback independently of the API listener.

## 8. Responsive and motion

At 900px metrics become two columns; at 760px navigation becomes an icon rail;
at 480px navigation becomes horizontal and the settings categories become tabs.
Inputs and localized text wrap without horizontal clipping. Normal page
navigation is immediate. Pointer presses scale to 0.96 for 100ms; keyboard
activation has no animation. Reduced motion disables press transforms.

## 9. Implementation guide

Use vanilla CSS only, split by base controls, layout, workspace, settings, and
responsive behavior. Example row: white background, 13px weight-500 heading,
11px `#9A9A9A` description, 36px gray input, 8px radius, zero tracking.

Example navigation: 36px visible row, 20px Lucide icon, gap10, padding14,
radius10, 14px weight600 label, selected `#EBEBEB`.

Example activity: 12px text, 11px `#A8A8A8` time, 13px horizontal inset, 1px
`#EEEEEE` continuous outline, 12px group radius, and 23px between dates.
