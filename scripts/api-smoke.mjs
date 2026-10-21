import assert from "node:assert/strict";
import { readFile, writeFile } from "node:fs/promises";

const requestTimeoutMilliseconds = 45000;
const [controlPath, reportPath, onlyCheck] = process.argv.slice(2);
if (!controlPath || !reportPath) {
  throw new Error("Usage: node scripts/api-smoke.mjs <private-control-file> <report.json> [exact-check-name]");
}
const control = new URL(JSON.parse(await readFile(controlPath, "utf8")).url);
const privateToken = control.hash.slice(1);
const controlHeaders = { "X-Desktop-Token": privateToken };
const stateResponse = await fetch(control.origin + "/control/state", {
  headers: controlHeaders, signal: AbortSignal.timeout(requestTimeoutMilliseconds),
});
assert.equal(stateResponse.status, 200);
const state = await stateResponse.json();
assert.equal(state.service.running, true, "Start the API before testing");
assert.equal(state.config.mode, "anonymous", "Use an isolated anonymous configuration");
assert.equal(state.api_keys_configured, 0, "Use an isolated keyless configuration");
const results = [];

const get = (path, options = {}) => fetch(state.network.local_url + path, {
  ...options, signal: AbortSignal.timeout(requestTimeoutMilliseconds),
});
const post = (path, body) => get(path, {
  method: "POST", headers: { "Content-Type": "application/json" },
  body: typeof body === "string" ? body : JSON.stringify(body),
});
const chat = (extra = {}) => ({
  model: "gemini-flash-lite",
  messages: [{ role: "user", content: "What is 8+5? Reply with only the number." }],
  ...extra,
});

async function check(name, action) {
  if (onlyCheck && name !== onlyCheck) return;
  const start = Date.now();
  try {
    const evidence = await action();
    results.push({ name, status: "passed", duration_ms: Date.now() - start, evidence });
    console.log("PASS " + name);
  } catch (error) {
    const message = error.message.replaceAll(privateToken, "[redacted]");
    results.push({ name, status: "failed", duration_ms: Date.now() - start, error: message });
    console.log("FAIL " + name + ": " + message);
  }
  await writeFile(reportPath, JSON.stringify(results, null, 2));
}

function events(text) {
  return text.replaceAll("\r\n", "\n").split("\n\n").filter(Boolean).map((block) => {
    const lines = block.split("\n");
    const data = lines.filter((line) => line.startsWith("data: ")).map((line) => line.slice(6)).join("\n");
    return {
      event: lines.find((line) => line.startsWith("event: "))?.slice(7),
      data: data === "[DONE]" ? data : JSON.parse(data),
    };
  });
}

await check("management requires private token", async () => {
  const response = await fetch(control.origin + "/control/state");
  assert.equal(response.status, 401);
});
await check("management state is authenticated and noncacheable", async () => {
  assert.equal(stateResponse.headers.get("cache-control"), "no-store");
  assert.equal(state.accounts.length, 0);
});
await check("health endpoint", async () => {
  const response = await get("/health");
  assert.equal(response.status, 200);
  assert.equal((await response.json()).status, "ok");
});
await check("model list", async () => {
  const response = await get("/v1/models");
  assert.equal(response.status, 200);
  const body = await response.json();
  assert.equal(body.data.length, state.models.length);
  return { model_count: body.data.length };
});
await check("unknown endpoint", async () => {
  assert.equal((await get("/missing")).status, 404);
});
await check("wrong HTTP method", async () => {
  assert.equal((await get("/v1/chat/completions")).status, 405);
});
const invalid = [
  ["invalid JSON", "{"],
  ["missing messages", { model: "gemini-flash-lite" }],
  ["empty messages", chat({ messages: [] })],
  ["unknown model", chat({ model: "missing-model" })],
  ["trailing JSON", JSON.stringify(chat()) + "\n{}"],
  ["unsupported image", chat({ messages: [{ role: "user", content: [
    { type: "image_url", image_url: { url: "data:image/png;base64,aA==" } },
  ] }] })],
];
for (const [name, body] of invalid) {
  await check(name + " is rejected", async () => {
    const response = await post("/v1/chat/completions", body);
    assert.equal(response.status, 400);
    assert.ok((await response.json()).error.message);
  });
}
await check("local token estimation is marked", async () => {
  const response = await post("/v1/messages/count_tokens", chat());
  assert.equal(response.status, 200);
  const body = await response.json();
  assert.equal(body.estimated, true);
  assert.ok(body.input_tokens > 0);
  return body;
});
for (const path of ["/v1/chat/completions", "/v1/messages"]) {
  await check(path + " reports account requirement", async () => {
    const response = await post(path, chat({ model: "gemini-4-argon-high" }));
    assert.equal(response.status, 502);
    const body = await response.json();
    assert.match(body.error.message, /此模型需要登录会话，当前为匿名模式/);
    return { error: body.error.message };
  });
}
await check("real anonymous OpenAI answer and route", async () => {
  const response = await post("/v1/chat/completions", chat({ temperature: 0 }));
  assert.equal(response.status, 200);
  const body = await response.json();
  assert.equal(body.choices[0].message.content.trim(), "13");
  assert.equal(response.headers.get("x-gemini-actual-mode"), "6");
  assert.match(response.headers.get("x-gemini-unsupported-parameters"), /temperature/);
  assert.equal(body.usage.estimated, true);
  return { answer: body.choices[0].message.content, actual_mode: body.gemini.actual_mode };
});
await check("real anonymous OpenAI stream", async () => {
  const response = await post("/v1/chat/completions", chat({ stream: true }));
  assert.equal(response.status, 200);
  const data = events(await response.text());
  assert.equal(data.at(-1).data, "[DONE]");
  const upstreamError = data.find((item) => item.data?.error)?.data.error;
  assert.equal(upstreamError, undefined, JSON.stringify(upstreamError));
  const text = data.filter((item) => item.data !== "[DONE]")
    .map((item) => item.data.choices?.[0]?.delta?.content || "").join("");
  assert.equal(text.trim(), "13");
  return { answer: text, event_count: data.length };
});
await check("real anonymous Anthropic answer", async () => {
  const response = await post("/v1/messages", chat({ max_tokens: 32 }));
  assert.equal(response.status, 200);
  const body = await response.json();
  const text = body.content.filter((block) => block.type === "text").map((block) => block.text).join("");
  assert.equal(text.trim(), "13");
  assert.equal(response.headers.get("x-gemini-actual-mode"), "6");
  return { answer: text, stop_reason: body.stop_reason };
});
await check("real anonymous Anthropic stream", async () => {
  const response = await post("/v1/messages", chat({ stream: true, max_tokens: 32 }));
  assert.equal(response.status, 200);
  const data = events(await response.text());
  const upstreamError = data.find((item) => item.event === "error")?.data;
  assert.equal(upstreamError, undefined, JSON.stringify(upstreamError));
  assert.equal(data.at(-1).event, "message_stop");
  const text = data.filter((item) => item.event === "content_block_delta")
    .map((item) => item.data.delta.text || "").join("");
  assert.equal(text.trim(), "13");
  return { answer: text, event_count: data.length };
});
assert.ok(results.length > 0, "Unknown check name: " + onlyCheck);
console.log(JSON.stringify({
  passed: results.filter((item) => item.status === "passed").length,
  failed: results.filter((item) => item.status === "failed").length,
}));
process.exitCode = results.some((item) => item.status === "failed") ? 1 : 0;
