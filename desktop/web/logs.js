import { createPage, escape, formattedTime, icon } from "./ui.js";

export function logs(workspace, context) {
  const page = createPage(workspace, "logs", `
    <div class="log-toolbar">
      <div class="search-field">${icon("search")}<input id="log-search" type="search" placeholder="搜索日志" aria-label="搜索日志"></div>
      <select id="log-level" aria-label="日志级别"><option value="">全部级别</option><option value="debug">Debug</option><option value="info">Info</option><option value="warn">Warn</option><option value="error">Error</option></select>
      <label class="checkbox-row compact"><input id="log-follow" type="checkbox" checked><span>跟随最新</span></label>
      <button id="download-logs" class="icon-button" type="button" title="导出日志" aria-label="导出日志">${icon("download")}</button>
    </div>
    <div class="section-heading log-count"><span id="log-count" class="meta"></span><span class="meta">北京时间</span></div>
    <div id="log-empty" class="empty-state">${icon("scroll-text")}<span>暂无日志</span></div>
    <div id="log-region" class="table-scroll log-region" tabindex="0" aria-label="运行日志"><table class="log-table"><thead><tr><th>时间</th><th>级别</th><th>事件</th></tr></thead><tbody id="log-rows"></tbody></table></div>`);
  for (const id of ["log-search", "log-level"]) page.querySelector(`#${id}`).addEventListener("input", () => update(page, context.getState()));
  page.querySelector("#download-logs").addEventListener("click", () => download(context.getState().logs));
  return { update: (state) => update(page, state) };
}

function update(page, state) {
  const query = page.querySelector("#log-search").value.toLowerCase();
  const level = page.querySelector("#log-level").value;
  const entries = state.logs.filter((entry) => (!level || entry.level.toLowerCase() === level) && `${entry.message} ${entry.level}`.toLowerCase().includes(query));
  page.querySelector("#log-count").textContent = `${entries.length} 条${entries.length !== state.logs.length ? ` / 共 ${state.logs.length} 条` : ""}`;
  page.querySelector("#log-empty").hidden = entries.length > 0;
  page.querySelector("#log-empty span").textContent = state.logs.length ? "没有匹配的日志" : "暂无日志";
  page.querySelector("#log-region").hidden = entries.length === 0;
  const rows = page.querySelector("#log-rows");
  const signature = JSON.stringify(entries);
  if (rows.dataset.signature === signature) return;
  rows.dataset.signature = signature;
  rows.innerHTML = entries.map((entry) => `<tr><td class="time-cell">${escape(formattedTime(entry.time))}</td><td><span class="log-level ${entry.level === "error" ? "error-text" : ""}">${escape(entry.level.toUpperCase())}</span></td><td>${escape(entry.message)}</td></tr>`).join("");
  if (page.querySelector("#log-follow").checked) {
    const region = page.querySelector("#log-region");
    region.scrollTop = region.scrollHeight;
  }
}

function download(entries) {
  const blob = new Blob([JSON.stringify(entries, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = "gemini-free-logs.json";
  link.click();
  URL.revokeObjectURL(url);
}
