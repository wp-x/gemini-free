import argparse
import json
import re
import socket
from pathlib import Path
from urllib.parse import urlsplit

from playwright.sync_api import expect, sync_playwright

from ui_geometry import (account_selector_checks, copy_and_navigation_checks,
                         settings_navigation_checks, verify_geometry)

TIMEOUT_MS = 8000
MINIMUM_TEXT_CONTRAST = 4.5
VIEWPORTS = [(1080, 800), (700, 540), (375, 812), (1440, 900)]
VIEWS = ["overview", "probe", "models", "logs", "accounts", "settings", "keys"]
THEMES = ["light", "dark"]
LAYOUT = """() => {
  const visible = (e) => e.getClientRects().length > 0;
  const regions = [document.documentElement, document.querySelector('main'),
    ...document.querySelectorAll('.settings-body, .settings-workspace')].filter(visible);
  const overflowing = regions.filter(e => e.scrollWidth > e.clientWidth + 1);
  const images = [...document.images].filter(visible);
  const controls = [...document.querySelectorAll('button, select, input')]
    .filter(visible).filter(e => !e.closest('.table-scroll'))
    .filter(e => !['radio', 'checkbox', 'file'].includes(e.type));
  const clipped = controls.filter(e => {
    const r = e.getBoundingClientRect();
    const labelClipped = e.textContent.trim() && e.scrollWidth > e.clientWidth + 1;
    return r.left < -1 || r.right > innerWidth + 1 || labelClipped;
  });
  return {overflowing: overflowing.map(e => e.id || e.className || e.tagName),
    clipped: clipped.map(e => e.id || e.className),
    brokenImages: images.filter(e => !e.complete || !e.naturalWidth).length,
    scrollbar: getComputedStyle(document.querySelector('main')).scrollbarWidth,
    letters: getComputedStyle(document.documentElement).letterSpacing};
}"""


def check(results, name, action):
    try:
        action()
        results.append({"name": name, "status": "passed"})
    except Exception as error:
        results.append({"name": name, "status": "failed", "error": str(error)})
        raise


def navigate(page, view):
    if view in ("accounts", "settings", "keys"):
        if not page.locator(".settings-dialog").is_visible():
            page.locator("#header-settings").click()
        page.locator(f'.settings-dialog a[data-view="{view}"]').click()
    else:
        if page.locator(".settings-dialog").is_visible():
            page.locator(".close-dialog").click()
            expect(page.locator(".settings-dialog")).not_to_be_visible()
        page.locator(f'.sidebar a[data-view="{view}"]').click()
    expect(page.locator(f"#page-{view}")).to_be_visible()


def verify_layout(page):
    data = page.evaluate(LAYOUT)
    assert not data["overflowing"], data
    assert not data["clipped"], data
    assert data["brokenImages"] == 0, data
    assert data["letters"] in ("normal", "0px"), data
    assert data["scrollbar"] in ("none", ""), data
    verify_geometry(page)


def appearance_checks(page, results, output):
    for theme in THEMES:
        page.emulate_media(color_scheme=theme)
        for width, height in VIEWPORTS:
            page.set_viewport_size({"width": width, "height": height})
            for view in VIEWS:
                navigate(page, view)
                check(results, f"{theme} {width}x{height} {view}", lambda: verify_layout(page))
                if width in (1080, 375) and view in ("overview", "settings", "probe"):
                    page.screenshot(path=str(output / f"{view}-{theme}-{width}.png"), animations="disabled")


def settings_checks(page):
    navigate(page, "settings")
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    page.locator("#port").fill(str(port))
    page.locator('#settings-form button[type="submit"]').click()
    expect(page.locator("#unsaved-settings")).to_have_text("")
    page.locator("#proxy").fill("http://127.0.0.1:7890")
    with page.expect_response(lambda response: urlsplit(response.url).path == "/control/state"):
        pass
    expect(page.locator("#proxy")).to_have_value("http://127.0.0.1:7890")
    navigate(page, "accounts")
    navigate(page, "settings")
    expect(page.locator("#proxy")).to_have_value("http://127.0.0.1:7890")
    page.keyboard.press("Escape")
    expect(page.locator(".settings-dialog")).not_to_be_visible()
    page.locator("#toggle-service").click()
    expect(page.locator("#metric-running")).to_have_text("运行中")
    navigate(page, "settings")
    expect(page.locator("#lan-access")).to_be_disabled()
    expect(page.locator("#port")).to_have_attribute("readonly", "")
    page.locator("#settings-service").click()
    expect(page.locator("#lan-access")).to_be_enabled()
    expect(page.locator("#port")).not_to_have_attribute("readonly", "")
    page.locator("#lan-access").check()
    expect(page.locator("#host")).to_have_value("0.0.0.0")
    page.locator("#lan-access").uncheck()
    expect(page.locator("#host")).to_have_value("127.0.0.1")


def form_checks(page):
    navigate(page, "accounts")
    page.locator("#add-account").click()
    expect(page.locator("#cookie")).to_be_focused()
    page.locator("#cookie").fill("ui-draft-only")
    page.locator("#toggle-cookie").click()
    expect(page.locator("#cookie")).to_have_attribute("type", "text")
    page.locator("#toggle-cookie").click()
    expect(page.locator("#cookie")).to_have_attribute("type", "password")
    navigate(page, "keys")
    navigate(page, "accounts")
    expect(page.locator("#cookie")).to_have_value("ui-draft-only")
    page.locator("#cookie").fill("")
    navigate(page, "probe")
    expect(page.locator("#send-probe")).to_be_disabled()
    page.locator("#prompt").fill("只用于检查按钮状态，不发送到上游。")
    expect(page.locator("#send-probe")).to_be_enabled()
    page.locator("#probe-model").select_option("gemini-4-argon-high")
    page.locator("#send-probe").click()
    expect(page.locator("#probe-form [data-error]")).to_contain_text("需要登录会话")
    page.locator("#prompt").fill("")
    expect(page.locator("#send-probe")).to_be_disabled()
    navigate(page, "logs")
    page.locator("#log-search").fill("no-matching-ui-record")
    expect(page.locator("#log-empty")).to_be_visible()
    page.locator("#log-search").fill("")
    expect(page.locator("#log-region")).to_be_visible()


def contrast_ratio(foreground, background):
    def luminance(color):
        channels = [float(value) / 255 for value in re.findall(r"[\d.]+", color)[:3]]
        linear = [value / 12.92 if value <= 0.04045 else ((value + 0.055) / 1.055) ** 2.4
                  for value in channels]
        return sum(value * weight for value, weight in zip(linear, (0.2126, 0.7152, 0.0722)))
    values = sorted((luminance(foreground), luminance(background)))
    return (values[1] + 0.05) / (values[0] + 0.05)


def assert_button_contrast(button):
    colors = button.evaluate(r"""e => {
      const s = getComputedStyle(e);
      return {text:s.color, stops:s.backgroundImage.match(/rgb\([^)]+\)/g)};
    }""")
    assert colors["stops"], colors
    for stop in colors["stops"]:
        ratio = contrast_ratio(colors["text"], stop)
        assert ratio >= MINIMUM_TEXT_CONTRAST, (colors, ratio)


def placeholder_contrast(page):
    navigate(page, "logs")
    field = page.locator("#log-search")
    field.hover()
    page.emulate_media(reduced_motion="reduce")
    colors = field.evaluate("""e => ({
      text:getComputedStyle(e,'::placeholder').color,
      background:getComputedStyle(e).backgroundColor
    })""")
    ratio = contrast_ratio(colors["text"], colors["background"])
    assert ratio >= MINIMUM_TEXT_CONTRAST, (colors, ratio)
    page.emulate_media(reduced_motion="no-preference")


def confirmation_checks(page, output):
    navigate(page, "overview")
    for theme in THEMES:
        page.emulate_media(color_scheme=theme)
        page.evaluate("""() => {
          import('./dialogs.js').then(({confirmAction}) => {
            window.confirmation = confirmAction({title:'移除登录会话？',
              message:'将从本机移除该会话与凭据。',action:'移除会话'});
          });
        }""")
        dialog = page.locator(".confirm-dialog")
        expect(dialog).to_be_visible()
        button = dialog.locator(".danger-primary")
        assert_button_contrast(button)
        button.hover()
        expect(button).to_have_css("color", "rgb(255, 255, 255)")
        assert button.evaluate("(e)=>getComputedStyle(e).backgroundImage") != "none"
        assert_button_contrast(button)
        page.screenshot(path=str(output / f"confirmation-{theme}.png"), animations="disabled")
        dialog.get_by_role("button", name="取消", exact=True).click()
        expect(dialog).not_to_be_visible()
        assert page.evaluate("window.confirmation") is False


def run(options):
    control = json.loads(options.control.read_text())
    token = urlsplit(control["url"]).fragment
    results, errors = [], []
    options.output.mkdir(parents=True, exist_ok=True)
    with sync_playwright() as playwright:
        browser = getattr(playwright, options.browser).launch(headless=True)
        page = browser.new_page(viewport={"width": 1080, "height": 800}, color_scheme="light")
        page.set_default_timeout(TIMEOUT_MS)
        page.on("pageerror", lambda error: errors.append(str(error)))
        try:
            page.goto(control["url"], wait_until="networkidle")
            expect(page.locator("#metric-models")).to_have_text("14")
            appearance_checks(page, results, options.output)
            page.set_viewport_size({"width": 1080, "height": 800})
            check(results, "real settings, drafts and service controls", lambda: settings_checks(page))
            check(results, "forms, account requirement and log filtering", lambda: form_checks(page))
            check(results, "account selector control heights", lambda: account_selector_checks(page, navigate))
            check(results, "fixed settings navigation while scrolling",
                  lambda: settings_navigation_checks(page, options.output, navigate))
            check(results, "real address copy and log navigation", lambda: copy_and_navigation_checks(page, navigate))
            for theme in THEMES:
                page.emulate_media(color_scheme=theme)
                check(results, f"{theme} input hover text contrast", lambda: placeholder_contrast(page))
            check(results, "confirmation cancel and danger hover", lambda: confirmation_checks(page, options.output))
            page.emulate_media(reduced_motion="reduce")
            transition = page.locator(".metric").first.evaluate("(e)=>getComputedStyle(e).transitionDuration")
            assert transition == "0s", transition
            assert not errors, errors
            results.append({"name": "reduced motion and JavaScript errors", "status": "passed"})
        except Exception as error:
            if not results or results[-1]["status"] != "failed":
                results.append({"name": "test execution", "status": "failed", "error": str(error)})
        finally:
            browser.close()
    report = json.dumps({"browser": options.browser, "checks": results}, ensure_ascii=False, indent=2)
    (options.output / f"results-{options.browser}.json").write_text(report.replace(token, "[redacted]"))
    failures = sum(item["status"] == "failed" for item in results)
    print(json.dumps({"browser": options.browser, "passed": len(results)-failures, "failed": failures}))
    if failures:
        print(json.dumps(results[-1], ensure_ascii=False).replace(token, "[redacted]"))
    return bool(failures)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Exercise the real desktop management UI in an isolated test instance.")
    parser.add_argument("control", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--browser", choices=["chromium", "webkit"], default="chromium")
    raise SystemExit(run(parser.parse_args()))
