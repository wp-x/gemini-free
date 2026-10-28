import { writeState } from "./api.js";
import { createPage, icon, submit, syncOptions } from "./ui.js";
import { renderActivity } from "./activity.js";
import { bindEndpointCopy, renderEndpoints } from "./network.js";

export function overview(workspace, context) {
  const page = createPage(workspace, "overview", `
    <div class="metric-grid">
      <section class="metric"><span class="metric-label">运行状态</span><strong id="metric-running">读取中</strong><span id="metric-address" class="metric-detail"></span><span class="metric-art green">${icon("server")}</span></section>
      <section class="metric"><span class="metric-label">模型配置</span><strong><span id="metric-models">0</span><small> 个</small></strong><span class="metric-art gold">${icon("boxes")}</span></section>
      <section class="metric"><span class="metric-label">登录会话</span><strong><span id="metric-accounts">0</span><small> 个</small></strong><span class="metric-art violet">${icon("users-round")}</span></section>
      <section class="metric"><span class="metric-label">API 密钥</span><strong><span id="metric-keys">0</span><small> 个</small></strong><span class="metric-art blue">${icon("key-round")}</span></section>
    </div>
    <p id="service-error" class="form-error" role="alert" hidden></p>
    <section class="home-mode-section">
      <div class="section-heading"><h2>当前模式</h2><button id="manage-accounts" class="text-button" type="button">管理会话</button></div>
      <form id="mode-form" class="mode-form">
        <fieldset class="segmented" aria-label="请求模式">
          <label><input type="radio" name="mode" value="anonymous"><span>${icon("user-round")}匿名模式</span></label>
          <label><input type="radio" name="mode" value="account"><span>${icon("key-round")}登录会话</span></label>
        </fieldset>
        <div class="field mode-account" id="mode-account-field"><label class="sr-only" for="active-account">当前会话</label><select id="active-account" name="selected_account"></select></div>
        <button class="button" type="submit">${icon("check")}应用</button>
        <p data-error class="form-error" role="alert" hidden></p>
      </form>
    </section>
    <section class="endpoint-section">
      <div class="section-heading"><h2>API 地址</h2><button id="network-settings" class="text-button" type="button">局域网设置</button></div>
      <div id="home-endpoints"></div>
    </section>
    <section class="activity-section"><div class="section-heading"><h2>最近活动</h2><button id="open-logs" class="icon-button" type="button" title="查看日志" aria-label="查看日志">${icon("scroll-text")}</button></div><div id="activity-list"></div></section>`);
  const form = page.querySelector("form");
  form.addEventListener("input", () => { form.dataset.dirty = "true"; reflectMode(page, form); });
  submit(form, context, async (data) => {
    if (data.get("mode") === "account" && !data.get("selected_account")) throw new Error("请先添加并选择登录会话");
    const state = await writeState("/control/config", { ...context.getState().config, mode: data.get("mode"), selected_account: data.get("selected_account") || "" });
    form.dataset.dirty = "";
    return state;
  });
  page.querySelector("#manage-accounts").addEventListener("click", () => context.openSettings("accounts"));
  page.querySelector("#open-logs").addEventListener("click", () => context.navigate("logs"));
  page.querySelector("#network-settings").addEventListener("click", () => context.openSettings("settings"));
  bindEndpointCopy(page.querySelector("#home-endpoints"), context);
  return { update: (state) => update(page, state) };
}

function reflectMode(page, form) {
  const mode = new FormData(form).get("mode");
  page.querySelector("#mode-account-field").hidden = mode !== "account";
}

function update(page, state) {
  const { service, config } = state;
  page.querySelector("#metric-running").textContent = service.running ? "运行中" : "已停止";
  page.querySelector("#metric-running").className = service.running ? "positive-text" : "";
  page.querySelector("#metric-address").textContent = `${state.network.lan_enabled ? "局域网" : "本机"} · ${config.port}`;
  page.querySelector("#metric-models").textContent = state.models.length;
  page.querySelector("#metric-accounts").textContent = state.accounts.length;
  page.querySelector("#metric-keys").textContent = state.api_keys_configured;
  page.querySelector("#service-error").textContent = service.error;
  page.querySelector("#service-error").hidden = !service.error;
  const form = page.querySelector("form");
  syncOptions(form.selected_account, [{ value: "", label: "选择登录会话" }, ...state.accounts.map((account) => ({ value: account.id, label: account.name }))], config.selected_account);
  if (!form.dataset.dirty) {
    form.querySelector(`[name="mode"][value="${config.mode}"]`).checked = true;
    form.selected_account.value = config.selected_account;
  }
  reflectMode(page, form);
  renderEndpoints(page.querySelector("#home-endpoints"), state);
  renderActivity(page.querySelector("#activity-list"), state.logs);
}
