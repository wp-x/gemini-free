import { escape, icon } from "./ui.js";

const dateFormatter = new Intl.DateTimeFormat("zh-CN", {
  timeZone: "Asia/Shanghai", month: "long", day: "numeric", weekday: "long",
});
const timeFormatter = new Intl.DateTimeFormat("zh-CN", {
  timeZone: "Asia/Shanghai", hour: "2-digit", minute: "2-digit", hour12: false,
});

export function renderActivity(element, logs) {
  const signature = JSON.stringify(logs);
  if (element.dataset.signature === signature) return;
  element.dataset.signature = signature;
  if (!logs.length) {
    element.innerHTML = `<div class="empty-state">${icon("scroll-text")}<span>暂无活动</span></div>`;
    return;
  }
  const groups = new Map();
  for (const log of [...logs].reverse()) {
    const date = new Date(log.time);
    const label = Number.isNaN(date.getTime()) ? log.time : dateFormatter.format(date);
    if (!groups.has(label)) groups.set(label, []);
    groups.get(label).push({ ...log, time: Number.isNaN(date.getTime()) ? "" : timeFormatter.format(date) });
  }
  element.innerHTML = [...groups].map(([date, entries]) => `<section class="activity-group"><h3>${escape(date)}</h3><div class="activity-entries">${entries.map((entry) => `<article class="activity-entry${entry.level === "error" ? " activity-error" : ""}"><p>${escape(entry.message)}</p><time>${escape(entry.time)}</time></article>`).join("")}</div></section>`).join("");
}
