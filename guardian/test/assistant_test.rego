# Runs the shared case set (guardian/cases/cases.yaml, as data.cases) and the
# phone vectors (data.phone_vectors) through the host assistant's rules, with
# the booking system's live read replaced by each case's guest_permissions, sent as the wire.
package assistant_test

import rego.v1

import data.assistant

step(c) := {"intervention_point": c.point, "snapshot": snapshot(c)}

snapshot(c) := {
	"actor": {"user_id": data.cases.actors[c.actor]},
	"tool_call": {"name": c.tool, "args": object.get(c, "args", {})},
} if c.point == "pre_tool_call"

# A tool's result: the booking system's answer as the tool returns it, JSON text.
snapshot(c) := {
	"actor": {"user_id": data.cases.actors[c.actor]},
	"tool_call": {"name": c.tool},
	"tool_result": {"exit_status": object.get(c, "exit_status", "success"), "outputs": [{"value": result_text(c)}]},
} if c.point == "post_tool_call"

# OpenClaw's text for an MCP tool's structured result; `raw` gives the text as is.
result_text(c) := c.raw if c.raw
else := concat("", ["structuredContent:\n", json.marshal(c.result)])

# The booking system's answer as it arrives, built from the case's view of a guest: one
# entry per guest the step reaches (an `entry` replaces it whole, to send a
# broken one; `guests` gives a phone its own entry); an unreachable view is no
# answer at all.
view(c) := data.cases.guest_permissions[c.guest_permissions] if c.guest_permissions
else := {"reachable": false}

answer(c, phones) := null if not view(c).reachable

answer(c, phones) := object.union(
	{"guests": {p: entry_for(view(c), p) | some p in phones}},
	{k: v | some k, v in view(c); k == "auto_reply"},
) if view(c).reachable

entry_for(v, p) := e if {
	e := v.guests[p]
} else := entry(v)

entry(v) := v.entry if v.entry

entry(v) := {"references": v.references, "secrets": v.secrets} if not v.entry

outcome(c) := got if {
	phones := assistant.reached_phones with input as step(c)
		with data.pack as data.cases.pack
	v := assistant.verdict with input as step(c)
		with data.pack as data.cases.pack
		with data.assistant.guest_permissions as answer(c, phones)
	got := shaped(v)
}

shaped(v) := "allow" if v.decision == "allow"

shaped(v) := {"deny": v.reason} if v.decision == "deny"

shaped(v) := {"ask": v.reason} if v.decision == "escalate"

failures contains {"case": c.name, "want": c.expect, "got": got} if {
	some c in data.cases.cases
	got := outcome(c)
	got != c.expect
}

test_every_case if count(failures) == 0

test_case_names_are_unique if {
	count({c.name | some c in data.cases.cases}) == count(data.cases.cases)
}

test_phone_vectors if {
	every v in data.phone_vectors {
		phone_matches(v)
	}
}

phone_matches(v) if {
	v.e164 == null
	not assistant.normalized_phone(v.input)
}

phone_matches(v) if {
	v.e164 != null
	assistant.normalized_phone(v.input) == v.e164
}

# The booking system's answer as it arrives: one read keyed by each guest's phone.
wire := {"auto_reply": {"guests": true, "others": false}, "guests": {"+529980000102": {
	"references": [{"value": "E2E-1042", "own": true}, {"value": "E2E-1051", "own": false}],
	"secrets": [{"value": "7316", "allowed": false}],
}}}

wire_verdict(c) := v if {
	v := assistant.verdict with input as step(c)
		with data.pack as data.cases.pack
		with data.assistant.guest_permissions as wire
}

test_the_wire_shape_reaches_each_guest if {
	wire_verdict({"point": "pre_tool_call", "actor": "maria", "tool": "message", "args": {"message": "El código es 7316."}}) == {"decision": "deny", "reason": "other_persons_data"}
	wire_verdict({"point": "pre_tool_call", "actor": "maria", "tool": "message", "args": {"message": "Hola"}}) == {"decision": "allow"}
}

test_a_guest_the_answer_misses_counts_as_unreachable if {
	wire_verdict({"point": "pre_tool_call", "actor": "owner", "tool": "message", "args": {"target": "+529980000103", "message": "Hola"}}) == {"decision": "escalate", "reason": "pms_unreachable", "message": "Datos de reserva no disponibles."}
}

test_approval_identifies_one_stay if {
	stay := {"reservation_ref": "CC-1042", "guest_name": "María", "listing_name": "Casa Coral", "check_in_date": "2026-10-17", "check_out_date": "2026-10-20"}
	answer_with_stay := {"auto_reply": {"guests": false, "others": false}, "guests": {"+529980000102": object.union(wire.guests["+529980000102"], {"stay": stay})}}
	v := assistant.verdict with input as step({"point": "pre_tool_call", "actor": "maria", "tool": "message", "args": {"message": "Hola"}})
		with data.pack as data.cases.pack
		with data.assistant.guest_permissions as answer_with_stay
	v.message == "María · CC-1042 · Casa Coral · 17–20 oct"
	v.decision == "escalate"
}

test_stay_dates_across_months if {
	assistant.stay_dates("2026-10-30", "2026-11-02") == "30 oct – 2 nov"
	assistant.stay_dates("2026-12-30", "2027-01-02") == "30 dic – 2 ene"
}
