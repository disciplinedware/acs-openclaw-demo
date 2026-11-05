import assert from "node:assert/strict";
import { mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

// The talk's scenarios through the installed ACS adapter, signed with the stack's key, to the running Guardian,
// whose rules read the running fake PMS. Every case holds on any date and with either auto-reply switch.
// The adapter loads through jiti, as OpenClaw loads it.
const { createJiti } = await import("/app/node_modules/jiti/lib/jiti.mjs");
const { default: plugin } = await createJiti(import.meta.url).import("/opt/acs/adapters/openclaw/acs-plugin.ts");

function keyedStore() {
  const rows = new Map();
  return {
    async register(key, value) { rows.set(key, value); },
    async registerIfAbsent(key, value) {
      if (rows.has(key)) return false;
      rows.set(key, value);
      return true;
    },
    async lookup(key) { return rows.get(key); },
    async consume(key) {
      const value = rows.get(key);
      rows.delete(key);
      return value;
    },
    async delete(key) { rows.delete(key); },
    async entries() { return [...rows].map(([key, value]) => ({ key, value })); },
    async update(key, next) {
      const value = next(rows.get(key));
      if (value !== undefined) rows.set(key, value);
    },
  };
}

const stores = new Map();
const handlers = new Map();
let toolResult;
const errors = [];
plugin.register({
  runtime: { state: { openKeyedStore: ({ namespace }) => stores.get(namespace) ?? stores.set(namespace, keyedStore()).get(namespace) } },
  pluginConfig: {
    ...JSON.parse(readFileSync("/home/node/.openclaw/guardian.json", "utf8")),
    auditLog: path.join(mkdtempSync(path.join(tmpdir(), "acs-test-guardian-")), "audit.jsonl"),
  },
  logger: { error: (m) => errors.push(m), warn: () => {}, info: () => {} },
  on: (name, handler) => handlers.set(name, handler),
  registerAgentToolResultMiddleware: (handler) => { toolResult = handler; },
});
assert.deepEqual(errors, [], "the adapter refused the stack's guardian.json");

async function guestRun(phone) {
  const sessionId = crypto.randomUUID();
  const sessionKey = `agent:assistant:whatsapp:default:direct:${phone}`;
  const runId = `test-guardian-${sessionId}`;
  const admitted = await handlers.get("before_agent_run")(
    { prompt: "Hola", senderId: phone },
    { runId, agentId: "assistant", sessionKey, sessionId, senderId: phone, channel: "whatsapp", trigger: "user" },
  );
  assert.equal(admitted?.block, undefined, `the Guardian refused the run: ${JSON.stringify(admitted)}`);
  let n = 0;
  const call = (toolName, params) => {
    const toolCallId = `call-${++n}`;
    return handlers.get("before_tool_call")({ toolName, params, runId, toolCallId }, { runId, sessionKey, sessionId, toolCallId });
  };
  // A call the Guardian allowed, then its result as the booking system answers: what the model may read.
  call.result = async (toolName, params, answer) => {
    const toolCallId = `call-${++n}`;
    const before = await handlers.get("before_tool_call")({ toolName, params, runId, toolCallId }, { runId, sessionKey, sessionId, toolCallId });
    assert.equal(before?.block, undefined, `the Guardian refused the call: ${JSON.stringify(before)}`);
    await handlers.get("after_tool_call")?.({ toolName, params, runId, toolCallId, result: {} }, { runId, sessionKey, sessionId, toolCallId });
    // OpenClaw labels an MCP tool's structured result this way.
    const content = [{ type: "text", text: `structuredContent:\n${JSON.stringify(answer, null, 2)}` }];
    return toolResult({ toolCallId, toolName, args: params, isError: false, result: { content, details: undefined } }, { runtime: "openclaw", runId, sessionKey, sessionId });
  };
  return call;
}

const booking = (ref, phone) => ({ status: "completed", result: { reservations: [{ reservation_ref: ref, guest_name: "Test", guest_phone: phone }] } });

const call = await guestRun("+447700900198");
const cases = [
  ["a_public_guide_is_allowed", "pms__pms-listing-get_guide", { listing_id: "00000000-0000-4000-8000-000000000001" },
    (r) => assert.equal(r?.block, undefined, JSON.stringify(r))],
  ["another_phones_booking_is_denied", "pms__pms-reservation-find_by_phone", { guest_phone: "+447700900100" },
    (r) => assert.match(r?.block === true ? r.blockReason : "", /otro teléfono, chat o reserva/)],
  ["exec_is_denied", "exec", { command: "cat ~/.openclaw/.env" },
    (r) => assert.match(r?.block === true ? r.blockReason : "", /Solo el anfitrión puede usar esta herramienta/)],
  ["another_persons_phone_is_denied", "message", { action: "send", message: "Llama a Luis al +44 7700 900110." },
    (r) => assert.match(r?.block === true ? r.blockReason : "", /datos de otra persona/, JSON.stringify(r))],
  ["another_guests_reservation_is_denied", "message", { action: "send", message: "Las fechas de tu reserva CC-1043 son del 3 al 6 de noviembre." },
    (r) => assert.match(r?.block === true ? r.blockReason : "", /datos de otra persona/, JSON.stringify(r))],
  ["another_houses_door_code_is_denied", "message", { action: "send", message: "El código de la puerta es 7316." },
    (r) => assert.match(r?.block === true ? r.blockReason : "", /datos de otra persona/, JSON.stringify(r))],
  // An auto-reply switch that is off asks before the money rule does; talk_test.rego proves the money rule itself.
  ["a_price_waits_for_the_host", "message", { action: "send", message: "Una noche más cuesta 3200 MXN." },
    (r) => assert.match(r?.requireApproval?.description ?? "", /Respuestas automáticas apagadas|habla de dinero/, JSON.stringify(r))],
];
for (const [name, tool, params, check] of cases) {
  check(await call(tool, params));
  console.log(`PASS: ${name}`);
}

// A guest's booking lookup: the model reads only the sender's own bookings.
const lookup = "pms__pms-reservation-find_by_ref";
assert.equal(await call.result(lookup, { reservation_ref: "CC-2001" }, booking("CC-2001", "+447700900198")), undefined);
console.log("PASS: her_own_booking_reaches_the_model");
const withheld = await call.result(lookup, { reservation_ref: "CC-1043" }, booking("CC-1043", "+447700900107"));
assert.ok(withheld !== undefined && !JSON.stringify(withheld).includes("+447700900107"), JSON.stringify(withheld));
console.log("PASS: another_guests_booking_is_withheld");

// A house's access data: the booking system's live read decides whose it is; this guest has no stay.
const access = { status: "completed", result: { listing_name: "Casita Coral", door_code: "7316", wifi_name: "CasitaCoral", wifi_password: "arrecife-2026-b" } };
const accessWithheld = await call.result("pms__pms-listing-get_access", { listing_id: "2f6a9c1e-5b3d-4c8a-9e10-000000000002" }, access);
assert.ok(accessWithheld !== undefined && !JSON.stringify(accessWithheld).includes("7316"), JSON.stringify(accessWithheld));
console.log("PASS: another_houses_access_is_withheld");
