import assert from "node:assert/strict";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";

process.env.OPENCLAW_STATE_DIR = mkdtempSync(path.join(tmpdir(), "whatsapp-packaged-send-"));
const plugin =
  process.argv[2] ?? "/opt/openclaw-plugins/node_modules/@disciplinedware/whatsapp-for-business";
const { whatsappPlugin } = await import(pathToFileURL(`${plugin}/dist/channel-plugin-api.js`));
const cfg = { channels: { whatsapp: { allowFrom: ["+447700900001"] } } };
const actions = whatsappPlugin.actions;
assert.ok(actions.describeMessageTool({ cfg }).actions.includes("send"));
assert.equal(actions.supportsAction({ action: "send" }), true);
assert.equal(actions.resolveExecutionMode({ action: "send", cfg }), "gateway");
await assert.rejects(
  actions.handleAction({
    channel: "whatsapp",
    action: "send",
    cfg,
    accountId: "default",
    sessionKey: "agent:host:whatsapp:default:direct:+447700900001",
    params: { to: "+447700900002", message: "Package validation" },
  }),
  /not in the configured allowFrom list/,
);
console.log("PASS: packaged send handler, stock SDK imports, Gateway dispatch mode, target refusal");
