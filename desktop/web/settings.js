import { writeState } from "./api.js";
import { createPage, icon, submit } from "./ui.js";
import { bindEndpointCopy, isLANHost, renderEndpoints } from "./network.js";

export function settings(workspace, context) {
  const page = createPage(workspace, "settings", `
    <form id="settings-form">
      <section class="settings-group">
        <div class="section-heading"><h2 class="settings-group-title">${icon("server")}API 服务</h2><button id="settings-service" class="button" type="button"></button></div>
        <label class="preference-row switch-row"><span><strong>局域网访问</strong><span id="lan-description" class="preference-description">允许同一网络的设备连接</span></span><input id="lan-access" type="checkbox" role="switch" class="switch" aria-label="局域网访问"></label>
        <div class="preference-row"><label for="port"><strong>端口</strong><span>兼容接口共用</span></label><input id="port" name="port" type="number" min="1" max="65535" required></div>
        <div class="network-auth"><span id="network-auth" class="meta"></span><button id="network-keys" class="text-button" type="button">管理密钥</button></div>
        <details class="network-addresses"><summary>访问地址</summary><div id="settings-endpoints" class="settings-endpoints"></div></details>
      </section>
      <section class="settings-group">
        <h2 class="settings-group-title">${icon("settings-2")}上游连接</h2>
        <div class="preference-row"><label for="proxy"><strong>出站代理</strong><span>HTTP、HTTPS 或 SOCKS5</span></label><input id="proxy" name="proxy" type="url" placeholder="直连" autocomplete="off" spellcheck="false"></div>
        <div class="preference-row"><label for="timeout"><strong>请求超时</strong><span>等待上游响应的秒数</span></label><input id="timeout" name="timeout_seconds" type="number" min="1" required></div>
      </section>
      <details class="advanced-options"><summary>高级设置</summary>
        <div class="preference-row"><label for="host"><strong>监听地址</strong></label><input id="host" name="host" required autocomplete="off"></div>
        <div class="preference-row preference-wide"><label for="build"><strong>匿名网页构建标识</strong></label><input id="build" name="build" required spellcheck="false" autocomplete="off"></div>
      </details>
      <div class="form-footer"><p data-error class="form-error" role="alert" hidden></p><span id="unsaved-settings" class="meta"></span><button class="button primary" type="submit">${icon("check")}保存设置</button></div>
    </form>`);
  const form = page.querySelector("form");
  bindNetwork(page, context);
  form.addEventListener("input", () => { form.dataset.dirty = "true"; page.querySelector("#unsaved-settings").textContent = "有未保存的更改"; });
  submit(form, context, async (data) => {
    const state = await writeState("/control/config", settingsConfig(data, context.getState().config));
    form.dataset.dirty = "";
    page.querySelector("#unsaved-settings").textContent = "";
    return state;
  });
  return { update: (state) => update(page, state) };
}

function settingsConfig(data, config) {
  return {
    ...config,
    host: String(data.get("host")).trim(),
    port: Number(data.get("port")),
    proxy: String(data.get("proxy")).trim(),
    timeout_seconds: Number(data.get("timeout_seconds")),
    build: String(data.get("build")).trim(),
  };
}

function update(page, state) {
  const form = page.querySelector("form");
  if (!form.dataset.dirty) {
    for (const key of ["host", "port", "proxy", "timeout_seconds", "build"]) form.elements[key].value = state.config[key] ?? "";
    page.querySelector("#lan-access").checked = state.network.lan_enabled;
  }
  for (const input of [form.host, form.port]) input.readOnly = state.service.running;
  page.querySelector("#lan-access").disabled = state.service.running;
  page.querySelector("#lan-description").textContent = state.service.running ? "停止服务后可更改访问范围" : "允许同一网络的设备连接";
  page.querySelector("#network-auth").textContent = state.api_keys_configured ? `访问认证：${state.api_keys_configured} 个密钥` : "访问认证：未设置密钥";
  const button = page.querySelector("#settings-service");
  button.innerHTML = icon(state.service.running ? "square" : "play") + (state.service.running ? "停止服务" : "启动服务");
  button.className = `button${state.service.running ? "" : " primary"}`;
  button.title = state.service.running ? "停止服务" : "启动服务";
  button.setAttribute("aria-label", button.title);
  renderEndpoints(page.querySelector("#settings-endpoints"), state);
}

function bindNetwork(page, context) {
  const form = page.querySelector("form");
  page.querySelector("#lan-access").addEventListener("change", (event) => {
    form.host.value = event.target.checked ? "0.0.0.0" : "127.0.0.1";
  });
  form.host.addEventListener("input", () => { page.querySelector("#lan-access").checked = isLANHost(form.host.value.trim()); });
  page.querySelector("#settings-service").addEventListener("click", (event) => context.perform(event.currentTarget,
    () => writeState("/control/service", { action: context.getState().service.running ? "stop" : "start" })));
  page.querySelector("#network-keys").addEventListener("click", () => context.openSettings("keys"));
  bindEndpointCopy(page.querySelector("#settings-endpoints"), context);
}
export function accessKeys(workspace, context) {
  const page = createPage(workspace, "keys", `
    <div class="settings-group">
      <div class="preference-row"><span><strong>已保存密钥</strong><span class="preference-description">用于兼容 API 的访问认证</span></span><span id="key-count" class="meta"></span></div>
    </div>
    <form id="keys-form">
      <div class="field"><label for="api-keys">替换密钥，每行一个</label><textarea id="api-keys" name="api_keys" rows="5" placeholder="留空以保留现有密钥" autocomplete="off" spellcheck="false"></textarea></div>
      <label class="preference-row switch-row"><span><strong>移除现有密钥</strong></span><input name="clear_keys" type="checkbox" role="switch" class="switch"></label>
      <div class="form-footer"><p data-error class="form-error" role="alert" hidden></p><button class="button primary" type="submit">${icon("check")}保存密钥</button></div>
    </form>`);
  const form = page.querySelector("form");
  submit(form, context, async (data) => {
    const keys = String(data.get("api_keys")).split("\n").map((key) => key.trim()).filter(Boolean);
    if (keys.length && data.has("clear_keys")) throw new Error("替换密钥和移除密钥不能同时选择");
    const payload = keys.length ? { api_keys: keys } : data.has("clear_keys") ? { api_keys: [] } : {};
    const state = await writeState("/control/config", { ...context.getState().config, ...payload });
    form.reset();
    return state;
  });
  return { update: (state) => { page.querySelector("#key-count").textContent = `${state.api_keys_configured} 个`; } };
}
