import assert from "node:assert/strict";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";

const testRoot = mkdtempSync(path.join(tmpdir(), "whatsapp-packaged-history-"));
process.env.OPENCLAW_STATE_DIR = path.join(testRoot, "state");
const plugin =
  process.argv[2] ?? "/opt/openclaw-plugins/node_modules/@disciplinedware/whatsapp-for-business";
const sdk = (name) => import(`/app/dist/plugin-sdk/${name}.js`);
const { setRuntimeConfigSnapshot } = await sdk("runtime-config-snapshot");
const { resolveAgentRoute } = await sdk("routing");
const { getSessionEntryAsync, recordSessionMetaFromInbound, resolveStorePath } =
  await sdk("session-store-runtime");
const { readVisibleSessionTranscriptMessageEntries } = await sdk("session-transcript-runtime");
const { setWhatsAppRuntime } = await import(pathToFileURL(`${plugin}/dist/runtime-setter-api.js`));
const { recordWhatsAppConversationMessage } = await import(
  pathToFileURL(`${plugin}/dist/conversation-history-api.js`)
);

{
  const cfg = {
    agents: { list: [{ id: "assistant", default: true }, { id: "host" }] },
    session: {
      store: path.join(testRoot, "state/{agentId}/sessions.json"),
      dmScope: "per-account-channel-peer",
    },
  };
  setRuntimeConfigSnapshot(cfg);
  setWhatsAppRuntime({ channel: { session: { resolveStorePath, recordSessionMetaFromInbound } } });
  const guest = "+447700900002";
  // A send of the host's agent is copied into the guest's session once; the guest agent's own reply is not.
  const sent = (messageId, sourceSessionKey) =>
    recordWhatsAppConversationMessage({
      cfg,
      accountId: "default",
      peer: guest,
      messageId,
      text: "Check-in is at noon.",
      timestampMs: Date.now(),
      author: "agent",
      sourceSessionKey,
    });
  await sent("packaged-host-reply", `agent:host:whatsapp:default:direct:${guest}`);
  await sent("packaged-host-reply", `agent:host:whatsapp:default:direct:${guest}`);
  const route = resolveAgentRoute({
    cfg,
    channel: "whatsapp",
    accountId: "default",
    peer: { kind: "direct", id: guest },
  });
  assert.equal(route.agentId, "assistant");
  const scope = {
    agentId: route.agentId,
    sessionKey: route.sessionKey,
    storePath: resolveStorePath(cfg.session.store, { agentId: route.agentId }),
  };
  const session = await getSessionEntryAsync(scope);
  assert.ok(session?.sessionId);
  const entries = await readVisibleSessionTranscriptMessageEntries({
    ...scope,
    sessionId: session.sessionId,
  });
  await sent("packaged-guest-reply", route.sessionKey);
  const after = await readVisibleSessionTranscriptMessageEntries({
    ...scope,
    sessionId: session.sessionId,
  });
  assert.equal(entries.length, 1);
  assert.equal(after.length, 1);
  assert.equal(entries[0].message.customType, "whatsapp.history");
  assert.match(entries[0].message.content[0].text, /Business agent: Check-in is at noon\./);
  console.log(
    "PASS: packaged WhatsApp history writer, stock SDK, recipient session, repeated delivery, own reply not copied",
  );
}
