const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "main.js"), "utf8");
const hook = "https://api.cloudflare.com/client/v4/workers/builds/deploy_hooks/12345678-abcd";

function createPlugin(secret, request) {
  const notices = [];
  class Plugin {}
  const mock = {
    Plugin,
    PluginSettingTab: class {},
    Setting: class {},
    Notice: class { constructor(message) { notices.push(message); } },
    requestUrl: request,
  };
  const context = { module: { exports: {} }, require: () => mock, URL };
  vm.runInNewContext(source, context);
  const plugin = new context.module.exports();
  plugin.app = { secretStorage: { getSecret: () => secret } };
  return { plugin, notices };
}

test("posts the hook and reports acceptance, not deployment", async () => {
  let request;
  const { plugin, notices } = createPlugin(hook, async (options) => {
    request = options;
    return { status: 200, json: { success: true, result: { build_uuid: "build-id" } } };
  });
  await plugin.startBuild();
  assert.equal(request.url, hook);
  assert.equal(request.method, "POST");
  assert.match(notices[0], /request accepted/i);
  assert.doesNotMatch(notices[0], /deploy.*success/i);
});

test("rejects missing and unsafe URLs without a request", async () => {
  for (const value of [null, "http://api.cloudflare.com/client/v4/workers/builds/deploy_hooks/a", "https://other.example/client/v4/workers/builds/deploy_hooks/a", `${hook}?token=x`]) {
    let called = false;
    const { plugin, notices } = createPlugin(value, async () => { called = true; });
    await plugin.startBuild();
    assert.equal(called, false);
    assert.match(notices[0], /set the deploy hook/i);
  }
});

test("reports HTTP and network errors without exposing the hook", async () => {
  for (const request of [
    async () => ({ status: 403, json: { success: false } }),
    async () => { throw new Error(hook); },
    async () => ({ status: 200, json: { success: true, result: {} } }),
  ]) {
    const { plugin, notices } = createPlugin(hook, request);
    await plugin.startBuild();
    assert.match(notices[0], /request failed/i);
    assert.equal(notices[0].includes(hook), false);
  }
});

test("does not send a second request while the first is pending", async () => {
  let calls = 0;
  let finish;
  const pending = new Promise((resolve) => { finish = resolve; });
  const { plugin } = createPlugin(hook, async () => { calls++; return pending; });
  const first = plugin.startBuild();
  await plugin.startBuild();
  assert.equal(calls, 1);
  finish({ status: 200, json: { success: true, result: { build_uuid: "build-id" } } });
  await first;
});
