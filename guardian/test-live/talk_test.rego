# The talk's scenarios on the real rules and a running fake PMS: no answer is
# fabricated, the rules make their live guest_permissions read over HTTP
# (guardian/test-live.sh starts the fake PMS on the test bookings, moved so
# that CC-1042 arrives today).
package talk_test

import rego.v1

import data.assistant

maria := "+447700900100" # the stage guest: CC-1042 at Casa Coral, arrives today

stranger := "+447700900199" # no stay with the business

pack := object.union(data.cases.pack, {"pms_url": "http://fake-pms:8080", "connect_key": "test-key"})

verdict(actor, tool, args) := v if {
	v := assistant.verdict with input as {
		"intervention_point": "pre_tool_call",
		"snapshot": {"actor": {"user_id": actor}, "tool_call": {"name": tool, "args": args}},
	}
		with data.pack as pack
}

# Every reason the rules have to ask the host, not only the one the verdict names.
ask_reasons(actor, tool, args) := r if {
	r := assistant.ask_reasons with input as {
		"intervention_point": "pre_tool_call",
		"snapshot": {"actor": {"user_id": actor}, "tool_call": {"name": tool, "args": args}},
	}
		with data.pack as pack
}

# Scenario 2, check A: a lookup acts only for the person who wrote.
test_the_agent_may_not_look_up_another_phone if {
	verdict(stranger, "pms__pms-reservation-find_by_phone", {"guest_phone": "+529980000199"}) == {"decision": "deny", "reason": "not_the_sender"}
}

# Scenario 2, check B: a message to a guest carries no other person's data.
test_a_typo_in_a_reference_never_reaches_a_guest if {
	v := verdict(maria, "message", {"action": "send", "message": "Las fechas de tu reserva CC-1043 son del 3 al 6 de noviembre."})
	v == {"decision": "deny", "reason": "other_persons_data"}
}

test_another_guests_booking_never_reaches_a_guest if {
	v := verdict(maria, "message", {"action": "send", "message": "La reserva de María López es la CC-1045, llega mañana."})
	v == {"decision": "deny", "reason": "other_persons_data"}
}

test_another_houses_door_code_never_reaches_a_guest if {
	v := verdict(maria, "message", {"action": "send", "message": "El código de la puerta es 7316, Wi-Fi arrecife-2026-b."})
	v == {"decision": "deny", "reason": "other_persons_data"}
}

# A guest may not run a command.
test_a_guest_may_not_run_a_command if {
	verdict(maria, "exec", {"command": "cat ~/.openclaw/.env"}) == {"decision": "deny", "reason": "owner_only"}
}

# Scenario 3: a price waits for the host, and his card names the guest's stay
# as the fake PMS gives it.
test_a_price_waits_for_the_host if {
	v := verdict(maria, "message", {"action": "send", "message": "Una noche más cuesta 3200 MXN."})
	v.decision == "escalate"
	"important_answer" in ask_reasons(maria, "message", {"action": "send", "message": "Una noche más cuesta 3200 MXN."})
	startswith(v.message, "Maria Gonzalez · CC-1042 · Casa Coral · ")
}

# Scenario 1: a person without a stay never gets an automatic reply; the
# answer waits for the host. Both auto-reply switches are off in the test
# bookings, so a guest's routine answer waits too.
test_a_routine_answer_waits_for_the_host if {
	guest := verdict(maria, "message", {"action": "send", "message": "El check-in es a las 15:00."})
	guest.decision == "escalate"
	guest.reason == "auto_reply_guests_off"
	other := verdict(stranger, "message", {"action": "send", "message": "Hola, ¿en qué te ayudo?"})
	other.decision == "escalate"
	other.reason == "auto_reply_others_off"
}
