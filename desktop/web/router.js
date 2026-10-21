import { accounts } from "./accounts.js";
import { settings, accessKeys } from "./settings.js";
import { escape, icon } from "./ui.js";

const settingsPages = [
  { id: "accounts", title: "登录会话", icon: "users-round", mount: accounts },
  { id: "settings", title: "通用连接", icon: "settings-2", mount: settings },
  { id: "keys", title: "访问密钥", icon: "key-round", mount: accessKeys },
];

export function createRouter(options) {
  const dialog = document.createElement("dialog");
  dialog.className = "settings-dialog";
  dialog.setAttribute("aria-labelledby", "settings-title");
  dialog.innerHTML = `<div class="settings-shell"><aside class="settings-sidebar"><nav aria-label="设置分类">${settingsPages.map(navLink).join("")}</nav></aside><div class="settings-body"><header class="settings-header"><h1 id="settings-title" tabindex="-1" autofocus>设置</h1><button class="icon-button close-dialog" type="button" aria-label="关闭设置" title="关闭设置">${icon("close")}</button></header><div class="settings-workspace"></div></div></div>`;
  document.body.append(dialog);
  const controllers = settingsPages.map((page) => page.mount(dialog.querySelector(".settings-workspace"), options.context));
  const session = { dialog, trigger: null, page: "overview" };
  setupNavigation(options, session);
  dialog.querySelector(".close-dialog").addEventListener("click", () => close(options, session));
  dialog.addEventListener("cancel", (event) => { event.preventDefault(); close(options, session); });
  window.addEventListener("popstate", () => renderRoute(options, session));
  renderRoute(options, session);
  return {
    navigate: (id) => navigate(options, session, id),
    openSettings: (id = "settings") => navigate(options, session, id),
    update: (state) => { for (const controller of controllers) controller.update(state); },
  };
}

function navLink(page) {
  return `<a href="?view=${page.id}${escape(location.hash)}" data-view="${page.id}" title="${page.title}">${icon(page.icon)}<span>${page.title}</span></a>`;
}

function setupNavigation(options, session) {
  document.querySelector("#navigation").innerHTML = options.pages.filter((page) => page.id !== "logs").map(navLink).join("");
  document.querySelector("#utility-navigation").innerHTML = navLink({ id: "settings", title: "设置", icon: "settings-2" }) + navLink(options.pages.find((page) => page.id === "logs"));
  document.addEventListener("click", (event) => {
    const anchor = event.target.closest("a[data-view]");
    if (!anchor) return;
    event.preventDefault();
    navigate(options, session, anchor.dataset.view);
  });
}

function navigate(options, session, id) {
  const isSettings = settingsPages.some((page) => page.id === id);
  if (isSettings && !session.dialog.open) session.trigger = document.activeElement;
  const url = new URL(location.href);
  url.searchParams.set("view", id);
  const inDialog = isSettings && session.dialog.open;
  history[inDialog ? "replaceState" : "pushState"]({ settingsOpened: isSettings }, "", url);
  renderRoute(options, session);
}

function close(options, session) {
  if (history.state?.settingsOpened) {
    history.back();
    return;
  }
  const url = new URL(location.href);
  url.searchParams.set("view", session.page);
  history.replaceState(null, "", url);
  renderRoute(options, session);
}

function renderRoute(options, session) {
  const id = new URLSearchParams(location.search).get("view") || "overview";
  const setting = settingsPages.find((page) => page.id === id);
  const page = setting ? options.pages.find((item) => item.id === session.page) : options.pages.find((item) => item.id === id) || options.pages[0];
  session.page = page.id;
  document.querySelector("#page-title").textContent = page.title;
  for (const element of document.querySelectorAll("#workspace > .page")) element.hidden = element.id !== `page-${page.id}`;
  for (const anchor of document.querySelectorAll(".sidebar nav a")) anchor.setAttribute("aria-current", anchor.dataset.view === (setting ? "settings" : page.id) ? "page" : "false");
  if (!setting) {
    if (session.dialog.open) { session.dialog.close(); session.trigger?.focus({ preventScroll: true }); }
    return;
  }
  document.querySelector("#settings-title").textContent = setting.title;
  for (const element of session.dialog.querySelectorAll(".page")) element.hidden = element.id !== `page-${setting.id}`;
  for (const anchor of session.dialog.querySelectorAll("nav a")) anchor.setAttribute("aria-current", anchor.dataset.view === setting.id ? "page" : "false");
  if (!session.dialog.open) session.dialog.showModal();
}
