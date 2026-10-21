import { spawnSync } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const executionTimeoutMilliseconds = 45000;
const outputBufferBytes = 1024 * 1024;
const vmName = process.env.GEMINI_WINDOWS_VM || "Windows 11";
const projectRoot = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const homePrefix = path.join(process.env.HOME, "Code") + path.sep;
const sharedRoot = process.env.GEMINI_WINDOWS_SHARE ||
  (projectRoot.startsWith(homePrefix) ?
    "\\\\Mac\\Code\\" + projectRoot.slice(homePrefix.length).split(path.sep).join("\\") : "");
if (!sharedRoot) {
  throw new Error("Set GEMINI_WINDOWS_SHARE to the Windows path of this project");
}
const quote = (value) => "'" + value.replaceAll("'", "''") + "'";
const scriptPath = sharedRoot + "\\scripts\\windows-smoke.ps1";
const headless = process.argv.includes("--headless") ? " -Headless" : "";
const script = `$ErrorActionPreference='Stop'; $ProgressPreference='SilentlyContinue'; ` +
  `[Console]::OutputEncoding=[Text.Encoding]::UTF8; & ${quote(scriptPath)}${headless}`;
const result = spawnSync("prlctl", ["exec", vmName, "--current-user", "powershell.exe",
  "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand",
  Buffer.from(script, "utf16le").toString("base64")], {
  timeout: executionTimeoutMilliseconds,
  killSignal: "SIGKILL",
  encoding: "utf8",
  maxBuffer: outputBufferBytes,
});
if (result.stdout) process.stdout.write(result.stdout);
if (result.stderr) process.stderr.write(result.stderr);
if (result.error) {
  process.stderr.write(`${result.error.message}; prlctl signal=${result.signal}\n`);
}
process.exitCode = result.status ?? 1;
