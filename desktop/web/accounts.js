import { writeState } from "./api.js";
import { confirmAction } from "./dialogs.js";
import { badge, createPage, escape, formattedTime, icon, message, submit } from "./ui.js";

export function accounts(workspace, context) {
  const page = createPage(workspace, "accounts", `
    <section class="settings-section">
      <div class="section-heading account-toolbar"><span id="account-count" class="meta"></span><div class="section-actions"><button id="import-legacy" class="icon-button" type="button" title="从旧版配置指定的 Cookie 文件导入，不切换当前模式" aria-label="导入旧版会话">${icon("download")}</button><button id="add-account" class="button primary" type="button">${icon("plus")}新增会话</button></div></div>
      <div id="account-empty" class="empty-state">${icon("users-round")}<span>未添加登录会话</span></div>
      <div id="account-list"><div id="account-rows" class="session-list"></div></div>
    </section>
    <details id="account-disclosure" class="account-disclosure">
      <summary>添加登录会话</summary>
      <form id="account-form" autocomplete="off">
        <div class="form-grid">
          <div class="field full-width"><div class="field-heading"><label for="cookie">Cookie 或会话 JSON</label><button id="import-cookie" class="button" type="button">${icon("download")}导入文件</button><input id="cookie-file" type="file" accept=".json,.txt,application/json,text/plain" hidden></div><div class="secret-field"><input id="cookie" name="cookie" type="password" required spellcheck="false" autocomplete="new-password"><button id="toggle-cookie" class="icon-button" type="button" title="显示 Cookie" aria-label="显示 Cookie">${icon("eye")}</button></div><span id="cookie-source" class="meta"></span></div>
          <div class="field full-width"><label for="account-name">会话名称（可选）</label><input id="account-name" name="name" placeholder="自动命名" autocomplete="off"></div>
        </div>
        <details class="advanced-options"><summary>高级参数（可选）</summary><div class="form-grid">
          <div class="field full-width"><label for="auth-user">账号序号</label><input id="auth-user" name="auth_user" type="number" min="0" placeholder="0"></div>
          <div class="field"><label for="xsrf-token">XSRF token</label><input id="xsrf-token" name="xsrf_token" type="password" placeholder="自动获取" autocomplete="new-password" spellcheck="false"></div>
          <div class="field"><label for="account-build">网页构建标识</label><input id="account-build" name="build" placeholder="自动获取" autocomplete="off" spellcheck="false"></div>
        </div></details>
        <div class="form-footer"><p data-error class="form-error" role="alert" hidden></p><button id="cancel-account-update" class="button" type="button" hidden>取消</button><button class="button primary" type="submit">${icon("plus")}添加会话</button></div>
      </form>
    </details>`);
  const form = page.querySelector("form");
  bindAccounts(page, context);
  form.addEventListener("input", () => { form.dataset.dirty = "true"; });
  submit(form, context, async (data) => {
    const endpoint = form.dataset.accountId ? `/control/accounts/${encodeURIComponent(form.dataset.accountId)}/session` : "/control/accounts";
    const state = await writeState(endpoint, Object.fromEntries(data));
    resetAccountForm(page);
    page.querySelector("#account-disclosure").open = false;
    return state;
  });
  return { update: (state) => update(page, state) };
}

function bindAccounts(page, context) {
  const form = page.querySelector("form");
  page.querySelector("#import-cookie").addEventListener("click", () => page.querySelector("#cookie-file").click());
  page.querySelector("#cookie-file").addEventListener("change", (event) => importCookieFile(page, event.target));
  page.querySelector("#toggle-cookie").addEventListener("click", (event) => toggleCookie(form.cookie, event.currentTarget));
  page.querySelector("#import-legacy").addEventListener("click", (event) => {
    context.perform(event.currentTarget, () => writeState("/control/import-legacy", {}));
  });
  page.querySelector("#add-account").addEventListener("click", () => {
    resetAccountForm(page);
    page.querySelector("#account-disclosure").open = true;
    form.cookie.focus();
  });
  page.querySelector("#cancel-account-update").addEventListener("click", () => {
    resetAccountForm(page);
    page.querySelector("#account-disclosure").open = false;
  });
  page.querySelector("#account-rows").addEventListener("click", async (event) => {
    const button = event.target.closest("[data-action]");
    if (!button) return;
    if (button.dataset.action === "replace") {
      editAccount(page, context.getState().accounts.find((account) => account.id === button.dataset.id));
      return;
    }
    if (button.dataset.action === "delete") {
      const confirmed = await confirmAction({ title: "移除登录会话？", message: `将从本机移除“${button.dataset.name}”的会话与凭据。`, action: "移除会话" });
      if (!confirmed) return;
    }
    const endpoint = `/control/accounts/${encodeURIComponent(button.dataset.id)}`;
    context.perform(button, () => button.dataset.action === "sync" ? writeState(`${endpoint}/sync`, {}) : writeState(endpoint, undefined, "DELETE"));
  });
}

function resetAccountForm(page) {
  const form = page.querySelector("form");
  form.reset();
  form.dataset.dirty = "";
  delete form.dataset.accountId;
  form.querySelector('[name="name"]').readOnly = false;
  form.querySelector('[name="auth_user"]').readOnly = false;
  form.cookie.type = "password";
  const toggle = page.querySelector("#toggle-cookie");
  toggle.innerHTML = icon("eye");
  toggle.title = "显示 Cookie";
  toggle.setAttribute("aria-label", "显示 Cookie");
  page.querySelector("#cookie-source").textContent = "";
  page.querySelector("#account-disclosure > summary").textContent = "添加登录会话";
  page.querySelector("#cancel-account-update").hidden = true;
  form.querySelector('[type="submit"]').innerHTML = `${icon("plus")}添加会话`;
  message(form, "");
}

function editAccount(page, account) {
  if (!account) return;
  resetAccountForm(page);
  const form = page.querySelector("form");
  form.dataset.accountId = account.id;
  const name = form.querySelector('[name="name"]');
  name.value = account.name;
  name.readOnly = true;
  const authUser = form.querySelector('[name="auth_user"]');
  authUser.value = account.auth_user || "";
  authUser.readOnly = true;
  page.querySelector("#account-disclosure > summary").textContent = `更新 Cookie · ${account.name}`;
  page.querySelector("#cancel-account-update").hidden = false;
  form.querySelector('[type="submit"]').innerHTML = `${icon("check")}保存会话`;
  page.querySelector("#account-disclosure").open = true;
  form.cookie.focus();
}

async function importCookieFile(page, input) {
  const file = input.files[0];
  if (!file) return;
  const form = page.querySelector("form");
  try {
    form.cookie.value = await file.text();
    form.dataset.dirty = "true";
    page.querySelector("#cookie-source").textContent = file.name;
    message(form, "");
  } catch (error) {
    message(form, `文件读取失败：${error.message}`);
  }
  input.value = "";
}

function toggleCookie(input, button) {
  const visible = input.type === "password";
  input.type = visible ? "text" : "password";
  const label = visible ? "隐藏 Cookie" : "显示 Cookie";
  button.innerHTML = icon(visible ? "eye-off" : "eye");
  button.title = label;
  button.setAttribute("aria-label", label);
}

function accountRow(account) {
  const status = account.error ? "异常" : account.status || "未验证";
  const kind = account.error ? "error" : "neutral";
  return `<article class="session-row"><div class="session-avatar">${icon("user-round")}</div><div class="session-content">
    <strong>${escape(account.name)}</strong><span class="table-description">账号序号 ${escape(account.auth_user || "0")} · 参数同步 ${escape(formattedTime(account.refreshed_at))}</span>
    ${badge(status, kind)}${account.error ? `<span class="table-description error-text">${escape(account.error)}</span>` : ""}</div>
    <div class="row-actions"><button class="icon-button" data-action="replace" data-id="${escape(account.id)}" type="button" title="更新 Cookie" aria-label="更新 ${escape(account.name)} 的 Cookie">${icon("key-round")}</button><button class="icon-button" data-action="sync" data-id="${escape(account.id)}" type="button" title="同步参数" aria-label="同步 ${escape(account.name)} 的参数">${icon("refresh-cw")}</button><button class="icon-button danger" data-action="delete" data-id="${escape(account.id)}" data-name="${escape(account.name)}" type="button" title="移除会话" aria-label="移除 ${escape(account.name)}">${icon("trash-2")}</button></div>
  </article>`;
}

function update(page, state) {
  page.querySelector("#account-count").textContent = `${state.accounts.length} 个会话`;
  page.querySelector("#account-empty").hidden = state.accounts.length > 0;
  page.querySelector("#account-list").hidden = state.accounts.length === 0;
  const rows = page.querySelector("#account-rows");
  const signature = JSON.stringify(state.accounts);
  if (rows.dataset.signature === signature) return;
  rows.dataset.signature = signature;
  rows.innerHTML = state.accounts.map(accountRow).join("");
}
