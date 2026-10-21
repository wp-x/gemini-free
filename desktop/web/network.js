import { escape, icon } from "./ui.js";

export function bindEndpointCopy(container, context) {
  container.addEventListener("click", async (event) => {
    const button = event.target.closest("[data-copy-url]");
    if (!button) return;
    try {
      await navigator.clipboard.writeText(button.dataset.copyUrl);
      context.notice("地址已复制");
    } catch (error) {
      context.notice(`复制失败：${error.message}`, "error");
    }
  });
}

export function renderEndpoints(container, state) {
  const { network, service } = state;
  const groups = [{ interface: "本机", url: network.local_url }, ...network.lan];
  const signature = JSON.stringify({ network, running: service.running });
  if (container.dataset.signature === signature) return;
  container.dataset.signature = signature;
  container.innerHTML = groups.map(endpointGroup).join("") +
    (network.error ? `<p class="form-error" role="alert">${escape(network.error)}</p>` : "") +
    (network.lan_enabled && !network.lan.length && !network.error ? '<p class="inline-note">未检测到可用的局域网地址</p>' : "") +
    (!service.running ? '<p class="inline-note">服务已停止，启动后可连接</p>' : "");
}

function endpointGroup(endpoint) {
  return `<div class="endpoint-group"><h3>${escape(endpoint.interface)}</h3>${[
    { name: "OpenAI", path: "/v1" },
    { name: "Anthropic", path: "/v1/messages" },
  ].map(({ name, path }) => {
    const url = escape(endpoint.url + path);
    const label = escape(`复制 ${endpoint.interface} ${name} 地址`);
    return `<div class="endpoint-row"><span>${name}</span><code>${url}</code><button class="icon-button" data-copy-url="${url}" type="button" title="${label}" aria-label="${label}">${icon("copy")}</button></div>`;
  }).join("")}</div>`;
}

export function isLANHost(host) {
  return host !== "localhost" && host !== "::1" && !host.startsWith("127.");
}
