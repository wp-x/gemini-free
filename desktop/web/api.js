const token = location.hash.slice(1);

export async function request(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: {
      "X-Desktop-Token": token,
      ...(options.body ? { "Content-Type": "application/json" } : {}),
    },
    cache: "no-store",
  });
  if (!response.ok) {
    const body = await response.json();
    throw new Error(body.error || `HTTP ${response.status}`);
  }
  return response;
}

export async function readState() {
  return (await request("/control/state")).json();
}

export async function writeState(path, body, method = "POST") {
  const response = await request(path, { method, body: JSON.stringify(body) });
  return response.json();
}

function parseEvent(raw) {
  const lines = raw.split("\n");
  const event = lines.find((line) => line.startsWith("event:"))?.slice(6).trim();
  const data = lines.filter((line) => line.startsWith("data:")).map((line) => line.slice(5).trimStart()).join("\n");
  return data ? { event: event || "message", data: JSON.parse(data) } : null;
}

export async function probe(options) {
  const response = await request("/control/probe", {
    method: "POST",
    body: JSON.stringify(options.body),
    signal: options.signal,
  });
  if (!response.body) throw new Error("管理服务没有返回响应流");
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let completed = false;
  while (true) {
    const chunk = await reader.read();
    buffer += decoder.decode(chunk.value, { stream: !chunk.done }).replace(/\r\n/g, "\n");
    const events = buffer.split("\n\n");
    buffer = events.pop();
    for (const raw of events) {
      const item = parseEvent(raw);
      if (!item) continue;
      if (item.event === "error") throw new Error(item.data.error);
      if (item.event === "result") completed = true;
      options.onEvent(item);
    }
    if (chunk.done) break;
  }
  if (!completed) throw new Error("响应流已结束，但未收到完成结果");
}
