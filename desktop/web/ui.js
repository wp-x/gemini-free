export function icon(name, extra = "") {
  return `<svg class="icon ${extra}" aria-hidden="true"><use href="./assets/icons.svg#${name}"></use></svg>`;
}

export function escape(value = "") {
  return String(value).replace(/[&<>"']/g, (character) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  })[character]);
}

export function badge(text, kind = "neutral") {
  return `<span class="badge ${kind}"><span class="status-dot"></span>${escape(text)}</span>`;
}

export function message(form, text) {
  const element = form.querySelector("[data-error]");
  element.textContent = text;
  element.hidden = !text;
}

export function busy(element, value) {
  element.disabled = value;
  element.setAttribute("aria-busy", String(value));
}

export function modelOptions(models, selected) {
  return models.map((model) => `<option value="${escape(model.id)}"${model.id === selected ? " selected" : ""}>${escape(model.id)}${model.requires_account ? " · 登录会话" : ""}</option>`).join("");
}

export function syncOptions(select, items, selected) {
  const markup = items.map((item) => `<option value="${escape(item.value)}">${escape(item.label)}</option>`).join("");
  if (select.dataset.options === markup) return;
  const value = select.dataset.initialized ? select.value : selected;
  select.innerHTML = markup;
  select.value = value;
  select.dataset.options = markup;
  select.dataset.initialized = "true";
}

export function formattedTime(value) {
  if (!value) return "尚未刷新";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat("zh-CN", {
    timeZone: "Asia/Shanghai", month: "2-digit", day: "2-digit",
    hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false,
  }).format(date);
}

export function createPage(workspace, id, markup) {
  const section = document.createElement("section");
  section.id = `page-${id}`;
  section.className = "page";
  section.setAttribute("aria-label", id);
  section.innerHTML = markup;
  workspace.append(section);
  return section;
}

export function submit(form, context, action) {
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    message(form, "");
    const button = form.querySelector('[type="submit"]');
    busy(button, true);
    try {
      const state = await action(new FormData(form));
      if (state) context.setState(state);
      context.notice("已保存");
    } catch (error) {
      message(form, error.message);
    } finally {
      busy(button, false);
    }
  });
}
