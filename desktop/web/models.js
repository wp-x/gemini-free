import { writeState } from "./api.js";
import { badge, createPage, escape, icon, submit, syncOptions } from "./ui.js";

export function models(workspace, context) {
  const page = createPage(workspace, "models", `
    <form id="model-form" class="inline-form">
      <div class="field grow"><label for="default-model">默认模型</label><select id="default-model" name="default_model"></select></div>
      <button class="button primary" type="submit">${icon("check")}保存</button>
      <p data-error class="form-error full-width" role="alert" hidden></p>
    </form>
    <section class="content-section">
      <div class="section-heading"><h2>模型与路由</h2><span id="model-count" class="meta"></span></div>
      <p class="inline-note">模型 ID 是本地别名，实际能力与路由由上游响应确认。</p>
      <div class="table-scroll" tabindex="0" role="region" aria-label="模型列表"><table class="model-table"><thead><tr><th>模型 ID</th><th>请求路由</th><th>访问条件</th><th>验证状态</th></tr></thead><tbody id="model-rows"></tbody></table></div>
    </section>`);
  const form = page.querySelector("form");
  form.addEventListener("input", () => { form.dataset.dirty = "true"; });
  submit(form, context, async (data) => {
    const state = await writeState("/control/config", { ...context.getState().config, default_model: data.get("default_model") });
    form.dataset.dirty = "";
    return state;
  });
  return { update: (state) => update(page, state) };
}

function update(page, state) {
  const form = page.querySelector("form");
  syncOptions(form.default_model, state.models.map((model) => ({ value: model.id, label: model.id })), state.config.default_model);
  if (!form.dataset.dirty) form.default_model.value = state.config.default_model;
  page.querySelector("#model-count").textContent = `${state.models.length} 个模型`;
  const signature = JSON.stringify(state.models);
  const rows = page.querySelector("#model-rows");
  if (rows.dataset.signature === signature) return;
  rows.dataset.signature = signature;
  rows.innerHTML = state.models.map((model) => `<tr><td><strong class="model-id">${escape(model.id)}</strong><span class="table-description">${escape(model.description)}</span></td><td><code>${escape(model.mode)}</code></td><td>${badge(model.requires_account ? "登录会话" : "匿名 / 登录")}</td><td class="verification">${escape(model.verification || "尚未验证")}</td></tr>`).join("");
}
