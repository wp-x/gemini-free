import { probe } from "./api.js";
import { badge, busy, createPage, icon, message, syncOptions } from "./ui.js";

export function question(workspace, context) {
  const page = createPage(workspace, "probe", `
    <form id="probe-form">
      <div class="form-grid">
        <div class="field"><label for="probe-model">请求模型</label><select id="probe-model" name="model" required></select></div>
        <div class="field"><label for="reasoning-effort">思考档位</label><select id="reasoning-effort" name="reasoning_effort"><option value="">模型默认</option value="low">低</option><option value="medium">中</option><option value="high">高</option></select></div>
      </div>
      <p id="probe-model-description" class="inline-note"></p>
      <div class="field prompt-field"><label for="prompt">问题</label><textarea id="prompt" name="prompt" rows="4" required placeholder="输入要测试的问题" spellcheck="false"></textarea></div>
      <div class="form-footer"><p data-error class="form-error" role="alert" hidden></p><span id="probe-mode" class="meta"></span><button id="cancel-probe" class="icon-button cancel-button" type="button" title="停止生成" aria-label="停止生成" hidden>${icon("square")}</button><button id="send-probe" class="icon-button send-button" type="submit" title="发送问题" aria-label="发送问题" disabled>${icon("arrow-up")}</button></div>
    </form>
    <section class="content-section result-section">
      <div class="section-heading"><h2>实际响应</h2><div id="probe-status">${badge("尚未测试")}</div></div>
      <dl class="result-meta"><div><dt>请求模型</dt><dd id="requested-model">尚未请求</dd></div><div><dt>实际 mode</dt><dd id="actual-mode">尚未返回</dd></div><div><dt>思考等级</dt><dd id="thinking-level">尚未返回</dd></div></dl>
      <div id="probe-empty" class="empty-state result-empty">${icon("messages-square")}<span>尚无测试结果</span></div>
      <pre id="probe-output" class="probe-output" aria-live="polite" aria-label="模型响应" hidden></pre>
    </section>`);
  const session = { controller: null, text: "", completed: false };
  const form = page.querySelector("form");
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    run(page, context, session);
  });
  form.model.addEventListener("change", () => reflectModel(page, context.getState()));
  form.prompt.addEventListener("input", () => updateSendButton(page));
  page.querySelector("#cancel-probe").addEventListener("click", () => session.controller?.abort());
  return { update: (state) => update(page, state) };
}

async function run(page, context, session) {
  if (session.controller) return;
  const form = page.querySelector("form");
  message(form, "");
  const model = context.getState().models.find((item) => item.id === form.model.value);
  if (model?.requires_account && context.getState().config.mode !== "account") {
    message(form, "此模型需要登录会话，当前为匿名模式");
    return;
  }
  session.controller = new AbortController();
  session.text = "";
  session.completed = false;
  prepare(page, form.model.value);
  setRunning(page, true);
  try {
    await probe({
      body: Object.fromEntries(new FormData(form)),
      signal: session.controller.signal,
      onEvent: (event) => receive(page, session, event),
    });
    page.querySelector("#probe-status").innerHTML = badge("已完成", "positive");
  } catch (error) {
    const cancelled = error.name === "AbortError";
    page.querySelector("#probe-status").innerHTML = badge(cancelled ? "已取消" : "请求失败", cancelled ? "neutral" : "error");
    if (!cancelled) message(form, error.message);
  } finally {
    session.controller = null;
    setRunning(page, false);
  }
}

function prepare(page, model) {
  page.querySelector("#requested-model").textContent = model;
  for (const id of ["actual-mode", "thinking-level"]) page.querySelector(`#${id}`).textContent = "等待返回";
  page.querySelector("#probe-status").innerHTML = badge("请求中");
  page.querySelector("#probe-empty").hidden = true;
  const output = page.querySelector("#probe-output");
  output.hidden = false;
  output.textContent = "";
}

function setRunning(page, running) {
  busy(page.querySelector("#send-probe"), running);
  page.querySelector("#cancel-probe").hidden = !running;
  page.querySelector("#probe-output").setAttribute("aria-busy", String(running));
  updateSendButton(page);
}

function updateSendButton(page) {
  const button = page.querySelector("#send-probe");
  button.disabled = button.getAttribute("aria-busy") === "true" || !page.querySelector("#prompt").value.trim();
}

function receive(page, session, event) {
  if (event.event === "delta") session.text += event.data.text || "";
  if (event.event === "result") {
    session.text = event.data.text;
    session.completed = true;
    page.querySelector("#requested-model").textContent = event.data.requested_model;
  }
  page.querySelector("#probe-output").textContent = session.text;
  if (event.data.actual_mode != null) page.querySelector("#actual-mode").textContent = String(event.data.actual_mode);
  if (event.data.thinking_level != null) page.querySelector("#thinking-level").textContent = String(event.data.thinking_level);
}

function reflectModel(page, state) {
  const form = page.querySelector("form");
  const model = state.models.find((item) => item.id === form.model.value);
  form.reasoning_effort.disabled = !model?.id.includes("argon");
  if (form.reasoning_effort.disabled) form.reasoning_effort.value = "";
  page.querySelector("#probe-model-description").textContent = model ? `${model.description} · ${model.verification || "尚未验证"}` : "无可用模型";
}

function update(page, state) {
  const form = page.querySelector("form");
  syncOptions(form.model, state.models.map((model) => ({ value: model.id, label: `${model.id}${model.requires_account ? " · 登录会话" : ""}` })), state.config.default_model);
  const account = state.accounts.find((item) => item.id === state.config.selected_account);
  page.querySelector("#probe-mode").textContent = state.config.mode === "account" ? `登录会话 · ${account?.name || "未选择"}` : "匿名模式";
  reflectModel(page, state);
}
