import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";

for (const scenario of [
  { name: "missing public API", publicUrl: "", serverUrl: undefined, valid: false },
  { name: "insecure API", publicUrl: "http://localhost:8080", serverUrl: undefined, valid: false },
  { name: "API with path", publicUrl: "https://api.example.com/v1", serverUrl: undefined, valid: false },
  { name: "different server API", publicUrl: "https://api.example.com", serverUrl: "https://other.example.com", valid: false },
  { name: "empty server API", publicUrl: "https://api.example.com", serverUrl: "", valid: false },
  { name: "matching API", publicUrl: "https://api.example.com", serverUrl: "https://api.example.com", valid: true },
  { name: "server fallback to public API", publicUrl: "https://api.example.com", serverUrl: undefined, valid: true },
]) {
  test(`Vercel configuration: ${scenario.name}`, () => {
    const environment = { ...process.env, VERCEL: "1", NEXT_PUBLIC_API_BASE_URL: scenario.publicUrl };
    if (scenario.serverUrl === undefined) delete environment.API_BASE_URL;
    else environment.API_BASE_URL = scenario.serverUrl;
    const result = spawnSync(process.execPath, ["--input-type=module", "-e", "await import('./next.config.ts')"], {
      cwd: new URL(".", import.meta.url),
      env: environment,
      encoding: "utf8",
    });
    assert.equal(result.status === 0, scenario.valid, result.stderr);
  });
}
