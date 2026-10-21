import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFile } from "node:fs/promises";

const requestTimeoutMilliseconds = 5000;
const vmTimeoutMilliseconds = 45000;
const questionTimeoutSeconds = 30;
const controlPath = process.argv[2];
const interfaceName = process.argv[3];
if (!controlPath || !interfaceName) {
  throw new Error("Usage: node scripts/lan-smoke.mjs <private-control-file> <interface>");
}
const control = JSON.parse(await readFile(controlPath, "utf8"));
const privateURL = new URL(control.url);
const response = await fetch(`${privateURL.origin}/control/state`, {
  headers: { "X-Desktop-Token": privateURL.hash.slice(1) },
  signal: AbortSignal.timeout(requestTimeoutMilliseconds),
});
assert.equal(response.status, 200);
const state = await response.json();
assert.equal(state.service.running, true, "Start the API before testing LAN access");
assert.equal(state.network.lan_enabled, true, "Save LAN access before testing");
const endpoint = state.network.lan.find((item) => item.interface === interfaceName);
assert.ok(endpoint, `No LAN address found for interface ${interfaceName}`);
assert.equal(state.api_keys_configured, 0, "This smoke check expects an isolated keyless test configuration");

const quote = (value) => "'" + value.replaceAll("'", "''") + "'";
const powershell = `
$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
$base=${quote(endpoint.url)}
$health=Invoke-RestMethod -Uri ($base+'/health') -TimeoutSec 5
$models=Invoke-RestMethod -Uri ($base+'/v1/models') -TimeoutSec 5
if ($models.data.Count -lt 1) { throw 'LAN model list is empty' }
$body=@{model='gemini-flash-lite';messages=@(@{role='user';content='What is 17+25? Reply with only the number.'})} | ConvertTo-Json -Depth 4
$answer=Invoke-RestMethod -Uri ($base+'/v1/chat/completions') -Method Post -ContentType 'application/json' -Body $body -TimeoutSec ${questionTimeoutSeconds}
$text=$answer.choices[0].message.content.Trim()
if ($text -ne '42') { throw ('Unexpected answer: '+$text) }
[PSCustomObject]@{health_reached=$true;model_count=$models.data.Count;answer=$text} | ConvertTo-Json -Compress
`;
const result = spawnSync("prlctl", ["exec", "Windows 11", "--current-user",
  "powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand",
  Buffer.from(powershell, "utf16le").toString("base64")], {
  encoding: "utf8", timeout: vmTimeoutMilliseconds,
});
if (result.error) throw result.error;
if (result.status !== 0) throw new Error(result.stderr || result.stdout);
const verification = JSON.parse(result.stdout.trim());
assert.equal(verification.health_reached, true);
assert.equal(verification.model_count, state.models.length);
assert.equal(verification.answer, "42");
console.log(JSON.stringify({ client: "Windows 11", interface: interfaceName, ...verification }));
