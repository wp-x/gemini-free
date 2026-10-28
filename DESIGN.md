# Gemini Free desktop design

## Current reference

The current visual reference is the user's revised
`/Users/mac/Downloads/client-design-v2.html`, received on October 21, 2026.
SHA256: `dd5efefd80089544052c379fa14545cf984716a320d1d2c3f4e7116bc7d6dab2`.

Static HTML and CSS comparison with `design/client-design.html` confirmed that
the revision changes the visual theme, while preserving markup, text and script.
The product adopts the revised theme through its existing stylesheet modules;
the reference's offline demonstration script is not part of the application.
The original reference file remains untouched.

Earlier AutoGLM investigations informed the initial layout. Logged-in evidence
was from version 1.7.0 (128), while the supplied DMG contained 1.19.0 (150).
Those observations are historical evidence, not the current design contract.
No private branding, account information or input-method features are copied.

## Theme tokens

`desktop/web/theme.css` owns the theme. Both appearances follow the operating
system through `prefers-color-scheme`; the HTML declares `light dark`.

| Role | Light | Dark |
| --- | --- | --- |
| Window shell | `#f3f4f6` | `#0b0c0e` |
| Workspace | `#ffffff` | `#131519` |
| Subtle surface | `#f9fafb` | `#181b20` |
| Input | `#f1f2f5` | `#1e2229` |
| Main text | `#111827` | `#f3f4f6` |
| Secondary text | `#5f6877` | `#9ca3af` |
| Muted text | `#626d7c` | `#8b95a5` |
| Focus and switches | `#4f46e5` | `#6366f1` |
| Positive text | `#047857` | `#34d399` |
| Error text | `#b91c1c` | `#f87171` |

Neutral surfaces, restrained borders and compact typography carry the hierarchy.
Overview metrics retain the reference's faint green, gold, violet and blue
accents. Functional focus and selected icons use the purple accent.

## Typography and geometry

Use the platform system stack with Chinese system font alternatives. No external
font request is required. Body text is 13px; page titles are 20px/650, section
headings 13.5px/600, control labels 12px, and metadata 11-12px. Tracking stays zero.
Status values use tabular numerals; reading and output surfaces allow selection.

| Element | Geometry |
| --- | --- |
| Main sidebar | 212px wide, 32px visible navigation rows |
| Window titlebar reserve | At least 34px, honoring the MyGo titlebar variable |
| Workspace | 14px radius, inset from the gray window shell |
| Main controls | 34px tall, 8px radius |
| Small surfaces | 6px radius |
| Overview metrics | 12px radius, four columns at standard desktop width |
| Settings dialog | Up to 820 by 620px, 16px radius |
| Settings sidebar | 196px at standard desktop width |
| Confirmation dialog | Up to 400px wide, shared 16px dialog radius |
| Settings close icon | 28px circle, extended invisible hit area |

Native window controls remain native. The frontend drag strip excludes the left
macOS controls and the right Windows button area. MyGo sets the initial window
to 1080 by 800, with a minimum of 700 by 540.

## Controls and feedback

Primary commands use the reference's dark gradient, thin border, restrained
shadow and subtle inset highlight. Dark appearance reverses primary foreground
and background. Secondary buttons use a quiet neutral gradient; utility icons
remain unfilled until interaction. Hover and pressed states share theme tokens.
Destructive confirmation uses a red gradient with white text in both appearances.
Disabled controls do not receive pressed scaling or enabled hover styles.

Selects share input height, radius and fill, with a vendored Lucide chevron.
Their native keyboard selection, disabled behavior and focus semantics remain.
Switches use the accent when checked. Segmented mode controls show the active
option on an inset workspace surface. Lucide icons are vendored; the existing
Gemini logo remains the product identity.

Focus uses a visible accent outline or field ring. Normal transitions are
140-200ms; pointer press scales to 0.97. Reduced motion removes transitions,
animations and pressed transforms, including pseudo-elements.

## Layout and real interactions

The shell fills the window. The document itself never scrolls; the main workspace,
settings workspace, output and tables scroll within their regions. Visible scrollbar
tracks remain hidden as previously requested, while wheel, touchpad and keyboard
scrolling continue to work. Table overflow stays keyboard focusable.

The main header retains an explicit Start/Stop Service action. Metrics display
actual runtime state, model count, account count and API key count. Mode selection
requires explicit application. Connection addresses come from actual listeners
and discovered interfaces; errors and offline states remain visible.

Settings remains a native HTML dialog with focus containment, Escape dismissal,
background scroll lock and focus restoration. Settings category changes preserve
drafts, and polling updates data without replacing edited fields. Account entry
is an inline disclosure; removing an account requires confirmation.

Cookie and session JSON entry, credential visibility, advanced session fields,
API keys, service startup and shutdown, log filters and question submission keep
their existing real implementations. No simulated result path is introduced.
Credentials are not stored in local storage.

The LAN switch controls the saved API listener scope. The service must stop
before changing listener scope or port. Management remains on loopback.
Discovered LAN addresses have separate OpenAI and Anthropic copy actions.

## Responsive behavior

At 900px, overview metrics become two columns and settings navigation narrows.
At 760px, the main sidebar becomes an icon rail and form fields wrap.
At 480px, main navigation becomes horizontal and settings categories become tabs.
The narrower layouts support browser validation and constrained windows; the
native desktop minimum remains 700 by 540.

## Alignment refinement, October 22, 2026

All visible SVG icon boxes retain their specified square dimensions rather than
inheriting an unrelated flex basis. Icon-only controls center their glyph in both
directions. Address copy actions and the activity log action share a right edge
and center guide. Section text actions align their text with the content edge.

The segmented mode control, account selector and apply action share the 34px
control height. Account toolbar controls use the same standard height, and
session row actions follow the right edge in both wide and wrapped layouts.
Navigation, page title and accessible labels consistently call the log view
"日志".

Settings has a fixed header with its title and close button. Only its workspace
scrolls, with clearance for keyboard focus rings. Scrolling expanded settings
does not move the close action out of the viewport.

## Deliberate reference adaptations

- Keep zero tracking instead of the reference's negative tracking.
- Keep text selection instead of globally disabling it.
- Hide scrollbar tracks rather than copying the reference's visible 6px tracks.
- Increase low-contrast small text, status text and destructive button colors.
- Use secondary text for placeholders so input hover remains readable.
- Preserve the red confirmation surface during hover instead of allowing a
  generic button hover rule to replace it with a neutral fill.
- Keep product scripts and real backend state rather than the offline demo.

## Implementation and verification

Styles remain vanilla CSS, split into theme, base controls, shell layout,
workspace, settings and responsive modules. Go and JavaScript business behavior
are unchanged by this visual revision.

`scripts/ui-smoke.py` exercises the real isolated management application in
Chromium: seven views, two appearances and four viewport sizes, plus settings,
service controls, account form state, log filtering and confirmation behavior.
Screenshots and the current verification report live in
`artifacts/ui-v2-20261021/`.
