import { readState, writeState } from "./api.js";
import { logs } from "./logs.js";
import { models } from "./models.js";
import { overview } from "./overview.js";
import { question } from "./probe.js";
import { createRouter } from "./router.js";
import { busy, icon } from "./ui.js";

const POLL_INTERVAL = 3000;
const NOTICE_DURATION = 5000;
const pages = [
  { id: "overview", title: "首页", icon: "layout-dashboard", mount: overview },
  { id: "probe", title: "问答", icon: "messages-square", mount: question },
  { id: "models", title: "模型", icon: "boxes", mount: models },
  { id: "logs", title: "运行日志", icon: "scroll-text", mount: logs },
];
let currentState;
let controllers = [];
let noticeTimer;
let reading = false;
let router;

function notice(text, kind = "positive") {
  const element = document.querySelector("#notice");
  const dialog = [...document.querySelectorAll("dialog[open]")].at(-1);
  (dialog || document.body).append(element);
  clearTimeout(noticeTimer);
  element.textContent = text;
  element.className = `notice ${kind}`;
  element.hidden = false;
  noticeTimer = setTimeout(() => { element.hidden = true; }, NOTICE_DURATION);
}

function setState(state) {
  currentState = state;
  if (!controllers.length) {
    const workspace = document.querySelector("#workspace");
    workspace.innerHTML = "";
    const context = { getState: () => currentState, setState, notice, perform,
      navigate: (id) => router.navigate(id), openSettings: (id) => router.openSettings(id) };
    controllers = pages.map((page) => page.mount(workspace, context));
    router = createRouter({ context, pages });
    controllers.push(router);
  }
  for (const controller of controllers) controller.update(state);
  const mode = state.config.mode === "account" ? "登录会话" : "匿名模式";
  const indicator = document.querySelector("#header-mode");
  indicator.innerHTML = icon(state.config.mode === "account" ? "user-round" : "eye-off");
  indicator.title = mode;
  indicator.setAttribute("aria-label", mode);
  connectionStatus(state.service.running ? "服务运行中" : "服务已停止", state.service.running ? "positive" : "");
  const toggle = document.querySelector("#toggle-service");
  toggle.innerHTML = icon(state.service.running ? "square" : "play") + (state.service.running ? "停止服务" : "启动服务");
  toggle.className = `button${state.service.running ? "" : " primary"}`;
  toggle.disabled = toggle.getAttribute("aria-busy") === "true";
}

function connectionStatus(label, kind) {
  const element = document.querySelector("#connection-status");
  element.title = label;
  element.setAttribute("aria-label", label);
  document.querySelector("#connection-dot").className = `status-dot ${kind}`;
}

async function perform(button, action) {
  busy(button, true);
  try { setState(await action()); }
  catch (error) { notice(error.message, "error"); }
  finally { busy(button, false); }
}

async function refresh() {
  if (reading) return;
  reading = true;
  const error = document.querySelector("#connection-error");
  try {
    setState(await readState());
    error.hidden = true;
  } catch (failure) {
    const text = `无法读取管理服务：${failure.message}`;
    if (error.hidden || error.textContent !== text) notice(text, "error");
    error.textContent = text;
    error.hidden = false;
    connectionStatus("管理连接异常", "error");
  } finally { reading = false; }
}

function initialize() {
  const button = document.querySelector("#refresh-state");
  button.innerHTML = icon("refresh-cw");
  document.querySelector("#header-settings").innerHTML = icon("settings-2");
  document.querySelector("#header-settings").addEventListener("click", () => router?.openSettings("settings"));
  button.addEventListener("click", refresh);
  document.querySelector("#toggle-service").addEventListener("click", (event) => perform(event.currentTarget,
    () => writeState("/control/service", { action: currentState.service.running ? "stop" : "start" })));
  document.querySelector(".skip-link").addEventListener("click", (event) => {
    event.preventDefault();
    const target = document.getElementById(event.currentTarget.dataset.skipTo);
    target.focus({ preventScroll: true });
  });
  document.addEventListener("pointerdown", (event) => event.target.closest("button, nav a")?.classList.add("pointer-press"));
  for (const type of ["pointerup", "pointercancel"]) document.addEventListener(type, () => {
    for (const element of document.querySelectorAll(".pointer-press")) element.classList.remove("pointer-press");
  });
  refresh();
  setInterval(refresh, POLL_INTERVAL);
}

initialize();
