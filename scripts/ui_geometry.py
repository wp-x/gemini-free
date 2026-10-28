from playwright.sync_api import expect

PIXEL_TOLERANCE = 0.6
GEOMETRY = """() => {
  const visible = e => e.getClientRects().length > 0;
  const icons = [...document.querySelectorAll('.icon')].filter(visible);
  const distorted = icons.filter(e => {
    const r = e.getBoundingClientRect();
    return Math.abs(r.width-r.height) > 0.6;
  }).map(e => e.parentElement.id || e.parentElement.className);
  const buttons = [...document.querySelectorAll('.icon-button, .mode-indicator')]
    .filter(visible);
  const offCenter = buttons.filter(e => {
    const icon = e.querySelector('.icon');
    if (!icon) return false;
    const a = e.getBoundingClientRect(), b = icon.getBoundingClientRect();
    return Math.abs(a.x+a.width/2-b.x-b.width/2) > 0.6 ||
      Math.abs(a.y+a.height/2-b.y-b.height/2) > 0.6;
  }).map(e => e.id || e.className);
  return {distorted, offCenter};
}"""


def verify_geometry(page):
    data = page.evaluate(GEOMETRY)
    assert not data["distorted"], data
    assert not data["offCenter"], data
    if page.locator("#page-overview").is_visible():
        verify_home_alignment(page)


def verify_home_alignment(page):
    data = page.evaluate("""() => {
      const selectors = ['#toggle-service', '#manage-accounts', '#network-settings',
        '#open-logs', '#home-endpoints .icon-button'];
      const rights = selectors.flatMap(s => [...document.querySelectorAll(s)]
        .map(e => e.getBoundingClientRect().right));
      const heights = ['.segmented', '#mode-form .button'].map(s =>
        document.querySelector(s).getBoundingClientRect().height);
      const textEdges = ['#manage-accounts', '#network-settings'].map(s => {
        const e = document.querySelector(s), range = document.createRange();
        range.selectNodeContents(e);
        return Math.abs(range.getBoundingClientRect().right-e.getBoundingClientRect().right);
      });
      return {rights, heights, textEdges};
    }""")
    assert max(data["rights"]) - min(data["rights"]) <= PIXEL_TOLERANCE, data
    assert max(data["heights"]) - min(data["heights"]) <= PIXEL_TOLERANCE, data
    assert max(data["textEdges"]) <= PIXEL_TOLERANCE, data


def account_selector_checks(page, navigate):
    navigate(page, "overview")
    page.locator('#mode-form input[value="account"]').check()
    expect(page.locator("#active-account")).to_be_visible()
    data = page.locator("#mode-form").evaluate("""e => {
      const selectors = ['.segmented', '#active-account', 'button[type="submit"]'];
      return selectors.map(s => e.querySelector(s).getBoundingClientRect().height);
    }""")
    assert max(data) - min(data) <= PIXEL_TOLERANCE, data
    page.locator('#mode-form input[value="anonymous"]').check()


def settings_navigation_checks(page, output, navigate):
    navigate(page, "settings")
    advanced = page.locator("#settings-form .advanced-options")
    if advanced.get_attribute("open") is None:
        advanced.locator("summary").click()
    for theme in ["light", "dark"]:
        page.emulate_media(color_scheme=theme, reduced_motion="reduce")
        for width, height in [(1080, 800), (700, 540), (375, 812)]:
            page.set_viewport_size({"width": width, "height": height})
            workspace = page.locator(".settings-workspace")
            workspace.evaluate("e => { e.scrollTop = 0; }")
            before = page.locator(".close-dialog").bounding_box()
            overflow = workspace.evaluate("e => e.scrollHeight > e.clientHeight")
            assert overflow, "Expanded settings must exercise a real scrolling region"
            workspace.evaluate("e => { e.scrollTop = e.scrollHeight; }")
            after = page.locator(".close-dialog").bounding_box()
            assert abs(before["y"] - after["y"]) <= PIXEL_TOLERANCE, (before, after)
            expect(page.locator("#settings-title")).to_be_in_viewport()
            expect(page.locator(".close-dialog")).to_be_in_viewport()
            expect(page.locator('#settings-form button[type="submit"]')).to_be_in_viewport()
            verify_geometry(page)
            page.screenshot(path=str(output / f"settings-fixed-header-{theme}-{width}.png"),
                            animations="disabled")
    advanced.locator("summary").click()
    page.set_viewport_size({"width": 1080, "height": 800})
    page.emulate_media(reduced_motion="no-preference")


def copy_and_navigation_checks(page, navigate):
    navigate(page, "overview")
    page.context.grant_permissions(["clipboard-read", "clipboard-write"])
    for button in page.locator("#home-endpoints [data-copy-url]").all():
        expected = button.get_attribute("data-copy-url")
        button.click()
        expect(page.locator("#notice")).to_have_text("地址已复制")
        assert page.evaluate("navigator.clipboard.readText()") == expected
    page.locator("#open-logs").click()
    expect(page.locator("#page-title")).to_have_text("日志")
    expect(page.locator('.sidebar a[data-view="logs"] span')).to_have_text("日志")
    expect(page.locator("#page-logs")).to_be_visible()
