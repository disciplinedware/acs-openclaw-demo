# The host's assistant: the 6 rules of the policy pack (README.md, "The rules").
# The sender is input.snapshot.actor.user_id, the phone the ACS
# adapter took from WhatsApp; the owner is the host's personal number, never
# the business number. A deny outranks a question to the owner, and a question
# outranks an allow. Every tool the pack does not name is refused.
package assistant

import rego.v1

pack := data.pack

snapshot := input.snapshot

point := input.intervention_point

actor := normalized_phone(snapshot.actor.user_id)

owner_phone := normalized_phone(pack.personal_phone)

owner_turn if actor == owner_phone

# OpenClaw names an MCP tool `<server>__<action>`, dots turned into hyphens.
tool := regex.replace(object.get(snapshot, ["tool_call", "name"], ""), `^pms__`, "")

args := object.get(snapshot, ["tool_call", "args"], {})

verdict := {"decision": "deny", "reason": reason} if {
	count(deny_reasons) > 0
	reason := min(deny_reasons)
} else := object.union({"decision": "escalate", "reason": reason}, approval_detail) if {
	count(ask_reasons) > 0
	reason := min(ask_reasons)
} else := {"decision": "allow"}

approval_detail := {"message": concat("\n", [recipient_line(phone, guest_permissions_for(phone)) | some phone in sort(reached_phones)])} if guest_message
else := {}

# The card already names the recipient; a line names its phone only when a message reaches several.
recipient_line(phone, d) := concat("", [phone_label(phone), "Datos de reserva no disponibles."]) if not d.reachable
else := sprintf("%s · %s · %s · %s", [stay.guest_name, stay.reservation_ref, stay.listing_name, stay_dates(stay.check_in_date, stay.check_out_date)]) if {
	stay := object.get(d, "stay", null)
	is_object(stay)
} else := concat("", [phone_label(phone), "Con reserva vigente; sin detalles."]) if holds_a_stay(d)
else := concat("", [phone_label(phone), "Sin reserva vigente."])

phone_label(phone) := sprintf("%s: ", [phone]) if count(reached_phones) > 1
else := ""

es_months := ["ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"]

# "4–6 nov", or "30 oct – 2 nov" across months.
stay_dates(check_in, check_out) := days_text(time.date(time.parse_ns("2006-01-02", check_in)), time.date(time.parse_ns("2006-01-02", check_out)))

days_text(a, b) := sprintf("%d–%d %s", [a[2], b[2], es_months[b[1] - 1]]) if a[1] == b[1]
else := sprintf("%d %s – %d %s", [a[2], es_months[a[1] - 1], b[2], es_months[b[1] - 1]])

guest_tools := {
	"pms-listing-get_guide",
	"pms-extra-list",
	"pms-reservation-find_by_phone",
	"pms-reservation-find_by_ref",
	"pms-listing-get_access",
	"pms-extra-order",
}

host_tools := {
	"pms-host-list_reservations",
	"pms-host-get_earnings",
	"pms-host-get_auto_reply",
	"pms-host-set_auto_reply",
}

file_tools := {"read", "write", "edit"}

memory_tools := {"memory_search", "memory_get"}

automation_tools := {"automations", "cron"}

# The guests' chats, read in the owner's turn.
session_tools := {"sessions_list", "sessions_history"}

# Web search, for the host's own questions. web_fetch stays unnamed, so the Guardian refuses it:
# it would send any URL, with whatever a guest's text slipped into it, to any host.
web_tools := {"web_search"}

# The host alone runs commands.
runtime_tools := {"exec"}

owner_only_tools := ((((host_tools | file_tools) | memory_tools) | automation_tools) | session_tools) | web_tools | runtime_tools

named_tools := (guest_tools | owner_only_tools) | {"message"}

deny_reasons contains "unknown_tool" if {
	point == "pre_tool_call"
	not tool in named_tools
}

# --- Phones: the forms a WhatsApp sender arrives in, normalized to E.164,
# proved by testdata/phones.yaml.

normalized_phone(raw) := out if {
	is_string(raw)
	out := normalized_e164(regex.replace(raw, `[ .()-]`, ""))
}

normalized_e164(s) := concat("", ["+52", s]) if regex.match(`^[0-9]{10}$`, s)

normalized_e164(s) := concat("", ["+", trim_prefix(s, "+")]) if regex.match(`^\+?52[0-9]{10}$`, s)

normalized_e164(s) := concat("", ["+52", substring(s, count(s) - 10, 10)]) if regex.match(`^\+?521[0-9]{10}$`, s)

normalized_e164(s) := s if {
	regex.match(`^\+[1-9][0-9]{7,14}$`, s)
	not startswith(s, "+52")
}

# A sender that is no phone (a WhatsApp privacy id, a broken envelope) is no
# one the rules can judge: every step of his turn is refused.
sender_is_a_phone if actor

deny_reasons contains "sender_not_a_phone" if not sender_is_a_phone

# The assistant writes to people only with the message tool, decided at its
# call: OpenClaw sends no other reply (messages.visibleReplies: message_tool,
# which openclaw/setup.sh refuses to drop).

# --- Rule 1: in a guest's turn a phone argument is the channel's sender: a
# guest tool's guest_phone, and a message's target, which may also be the
# owner. The owner's turn is not bound: the host directs his own assistant.

is_the_sender(raw) if normalized_phone(raw) == actor

# The guest tools that act for one person take that person's phone; a call
# without it is refused here, not left to the booking system.
phone_tools := {"pms-reservation-find_by_phone", "pms-extra-order"}

deny_reasons contains "not_the_sender" if {
	point == "pre_tool_call"
	not owner_turn
	tool in phone_tools
	not is_the_sender(object.get(args, "guest_phone", null))
}

# In a guest's turn a booking lookup returns only the sender's own bookings.
# Checked on the tool's result before the model reads it: each booking carries
# its guest's phone, so a lookup by another guest's reference is withheld.
reservation_lookups := {"pms-reservation-find_by_phone", "pms-reservation-find_by_ref"}

# The guest tools whose results are checked before the model reads them.
checked_results := reservation_lookups | {"pms-listing-get_access"}

deny_reasons contains "another_persons_reservation" if {
	point == "post_tool_call"
	not owner_turn
	tool in reservation_lookups
	some booking in returned_reservations
	not is_the_sender(object.get(booking, "guest_phone", null))
}

# In a guest's turn a house's access data reaches the model only when it is the
# sender's own. get_access takes a house, not a person, so its result names no
# guest; the booking system's live read for the sender marks a door code or
# Wi-Fi password allowed while a paid stay of the sender is inside its access
# window, among all the sender's bookings.
access_tool := "pms-listing-get_access"

deny_reasons contains "another_persons_access" if {
	point == "post_tool_call"
	not owner_turn
	tool == access_tool
	guest_permissions_for(actor).reachable
	some secret in returned_secrets
	not own_secret(secret)
}

# Without the booking system's answer nobody can tell whose the access data is.
deny_reasons contains "access_unverified" if {
	point == "post_tool_call"
	not owner_turn
	tool == access_tool
	count(returned_secrets) > 0
	not guest_permissions_for(actor).reachable
}

returned_secrets contains secret if {
	some answer in lookup_answers
	some field in ["door_code", "wifi_password"]
	secret := object.get(answer, ["result", field], "")
	is_string(secret)
	secret != ""
}

own_secret(secret) if {
	some s in guest_permissions_for(actor).secrets
	s.value == secret
	s.allowed == true
}

# A successful lookup whose answer cannot be read is withheld too: a booking or
# a secret in it could not be checked.
deny_reasons contains "unreadable_result" if {
	point == "post_tool_call"
	not owner_turn
	tool in checked_results
	object.get(snapshot, ["tool_result", "exit_status"], "") == "success"
	count(lookup_answers) == 0
}

# The bookings in a tool result. Each output holds the booking system's answer:
# an object, or text whose JSON starts at its first brace (OpenClaw labels an
# MCP tool's structured result `structuredContent:`).
returned_reservations contains booking if {
	some answer in lookup_answers
	some booking in object.get(answer, ["result", "reservations"], [])
}

lookup_answers contains answer if {
	some output in object.get(snapshot, ["tool_result", "outputs"], [])
	answer := result_body(output.value)
}

result_body(value) := value if is_object(value)

result_body(value) := json.unmarshal(body) if {
	is_string(value)
	start := indexof(value, "{")
	start >= 0
	body := substring(value, start, -1)
	json.is_valid(body)
}

# A guest's chat writes to that guest or to the owner; the owner's chat writes
# to anyone. A message without a target goes into the chat it runs in.
# OpenClaw's message tool names one recipient as target, to or channelId,
# and several as targets.
single_target_arguments := {"target", "to", "channelId"}

message_targets contains t if {
	tool == "message"
	some name in single_target_arguments
	t := object.get(args, name, null)
	t != null
}

message_targets contains t if {
	tool == "message"
	targets := object.get(args, "targets", [])
	is_array(targets)
	some t in targets
}

# A recipient list that is no array names recipients nobody can check.
deny_reasons contains "target_not_e164" if {
	point == "pre_tool_call"
	tool == "message"
	not is_array(object.get(args, "targets", []))
}

# The adapter delivers only to exactly the target the Guardian saw, so a
# target is written as WhatsApp delivers it: + and digits, or `owner`, the
# host, whose number the agent never sees (the WhatsApp plugin resolves it in
# its final send).
deny_reasons contains "target_not_e164" if {
	point == "pre_tool_call"
	some t in message_targets
	not e164_text(t)
}

# A target in E.164 form that names no phone (a +52 number of the wrong
# length) matches neither the sender nor the owner: it is refused, never
# left unchecked.
deny_reasons contains "target_not_e164" if {
	point == "pre_tool_call"
	some t in message_targets
	e164_text(t)
	not target_phone(t)
}

e164_text(t) if {
	is_string(t)
	regex.match(`^\+[1-9][0-9]{7,14}$`, t)
}

e164_text("owner")

# The phone a message target names: `owner` is the host.
target_phone(t) := owner_phone if t == "owner"

target_phone(t) := normalized_phone(t) if t != "owner"

deny_reasons contains "not_the_sender" if {
	point == "pre_tool_call"
	not owner_turn
	some t in message_targets
	not is_the_sender(t)
	not target_phone(t) == owner_phone
}

deny_reasons contains "not_the_sender" if {
	point == "pre_tool_call"
	not owner_turn
	tool == "message"
	not object.get(args, "channel", "whatsapp") in {"whatsapp"}
}

# --- Rule 2: the host's tools, memory, files and automations are the owner's
# alone, and the owner uses them freely; a guest turn may send or react. Commands
# are closed in OpenClaw's configuration, and no tool that changes it is named.

deny_reasons contains "owner_only" if {
	point == "pre_tool_call"
	tool in owner_only_tools
	not owner_turn
}

deny_reasons contains "guest_message_actions_only" if {
	point == "pre_tool_call"
	tool == "message"
	not owner_turn
	object.get(args, "action", "send") != "send"
	object.get(args, "action", "send") != "react"
}

# A file reaches a guest from a guest's chat only by web link or as a file a
# guest sent (OpenClaw's inbound media). Any other local path - the host's
# notes, the workspace, the state directory - is read from disk at delivery,
# where neither the model nor rule 4 sees its contents.
deny_reasons contains "local_file" if {
	point == "pre_tool_call"
	not owner_turn
	count(message_guests) > 0
	some file in sent_files
	not file_allowed(file)
}

# OpenClaw takes a file from many arguments (media, path, filePath, mediaUrl,
# fileUrl, image, their snake_case forms, a channel's own; in an attachment
# also url). A message that reaches a guest from a guest's chat may name only
# the message tool's send arguments below, so `media` and `attachments[].media`
# are its only files; any other argument is refused.
guest_send_arguments := {
	"action", "message", "target", "targets", "to", "channelId", "channel", "accountId",
	"media", "attachments", "filename", "caption", "contentType", "mimeType", "buffer",
	"replyTo", "threadId", "silent", "asDocument", "forceDocument", "gifPlayback",
	"effectId", "effect", "asVoice", "voiceText", "voiceProvider", "voiceId", "quoteText", "final", "requestApproval",
}

guest_react_arguments := {
	"action", "target", "to", "channelId", "channel", "accountId",
	"messageId", "message_id", "emoji", "final",
}

attachment_fields := {"type", "media", "name", "mimeType"}

deny_reasons contains "unknown_argument" if {
	point == "pre_tool_call"
	not owner_turn
	count(message_guests) > 0
	object.get(args, "action", "send") == "react"
	some name, _ in args
	not name in guest_react_arguments
}

deny_reasons contains "unknown_argument" if {
	point == "pre_tool_call"
	not owner_turn
	count(message_guests) > 0
	object.get(args, "action", "send") != "react"
	some name, _ in args
	not name in guest_send_arguments
}

deny_reasons contains "unknown_argument" if {
	point == "pre_tool_call"
	not owner_turn
	count(message_guests) > 0
	attachments := object.get(args, "attachments", [])
	not is_array(attachments)
}

deny_reasons contains "unknown_argument" if {
	point == "pre_tool_call"
	not owner_turn
	count(message_guests) > 0
	some attachment in object.get(args, "attachments", [])
	not is_object(attachment)
}

deny_reasons contains "unknown_argument" if {
	point == "pre_tool_call"
	not owner_turn
	count(message_guests) > 0
	some attachment in object.get(args, "attachments", [])
	is_object(attachment)
	some name, _ in attachment
	not name in attachment_fields
}

sent_files contains object.get(args, "media", null) if object.get(args, "media", null) != null

sent_files contains file if {
	attachments := object.get(args, "attachments", [])
	is_array(attachments)
	some attachment in attachments
	file := object.get(attachment, "media", null)
	file != null
}

file_allowed(file) if {
	is_string(file)
	regex.match(`^https?://`, file)
}

file_allowed(file) if {
	is_string(file)
	startswith(file, concat("", [trim_suffix(pack.inbound_media_dir, "/"), "/"]))
	not contains(file, "..")
	not contains(file, "%")
	not contains(file, "\\")
}

# --- Rules 3, 4 and 5: what a message to a guest may say. The booking system answers
# live (pms.guardian.guest_permissions) with the two auto-reply switches and every
# reference and secret of the business, the guest's own marked.

# One read per step, for every guest the step reaches.
guest_permissions := d if {
	count(reached_phones) > 0
	response := http.send({
		"method": "POST",
		"url": concat("", [trim_suffix(pack.pms_url, "/"), "/api/actions/pms.guardian.guest_permissions"]),
		"headers": {"Authorization": concat(" ", ["Bearer", pack.connect_key]), "Content-Type": "application/json"},
		"body": {"guest_phones": sort(reached_phones)},
		"timeout": pack.guest_permissions_timeout,
		"raise_error": false,
		"cache": false,
	})
	response.status_code >= 200
	response.status_code < 300
	d := response.body.result
} else := null

# What one guest may be told; a guest the answer misses counts as unreachable.
# An entry of the wrong shape counts as unreachable too.
guest_permissions_for(phone) := object.union(entry, {"reachable": true, "auto_reply": object.get(guest_permissions, "auto_reply", null)}) if {
	is_object(guest_permissions)
	entry := object.get(object.get(guest_permissions, "guests", {}), phone, null)
	is_object(entry)
	is_array(object.get(entry, "references", null))
	is_array(object.get(entry, "secrets", null))
} else := {"reachable": false}

# The guests a message reaches. A guest's chat: its sender, unless the message
# goes to the owner. The owner's chat: every target that is not the owner.
message_guests contains actor if {
	point == "pre_tool_call"
	tool == "message"
	not owner_turn
	not to_owner_only
}

to_owner_only if {
	count(message_targets) > 0
	every t in message_targets {
		target_phone(t) == owner_phone
	}
}

message_guests contains phone if {
	point == "pre_tool_call"
	owner_turn
	some t in message_targets
	phone := target_phone(t)
	phone != owner_phone
}

guest_message if count(message_guests) > 0

reached_phones contains phone if some phone in message_guests

# A guest's access lookup: the booking system answers for the sender.
reached_phones contains actor if {
	point == "post_tool_call"
	not owner_turn
	tool == access_tool
	count(returned_secrets) > 0
}

# Each guest a message or reply reaches, with the booking system's live read for him.
reached contains {"phone": phone, "guest_permissions": guest_permissions_for(phone)} if {
	some phone in reached_phones
}

# The text a call delivers: every string of its arguments at any depth, each
# on its own, so a value split across two fields passes alike (as across two messages). The
# recipients are not text: a target is the guest's own phone.
texts contains value if {
	guest_message
	point == "pre_tool_call"
	walk(args, [path, value])
	is_string(value)
	not path[0] in ((single_target_arguments | {"targets"}) | identifier_arguments)
}

# The message tool's arguments that name things rather than carry text the
# person reads: WhatsApp message ids hold digits that can read as a phone.
# Every other argument, a new one included, is scanned.
identifier_arguments := {
	"action", "channel", "accountId", "replyTo", "threadId",
	"effectId", "voiceId", "voiceProvider", "contentType", "mimeType",
	"messageId", "message_id",
}

# Rule 3: the business's two auto-reply switches, read live: one for its
# guests (a person with a current or upcoming stay), one for everyone else.
# A message to a person whose switch is off waits for the owner, from either
# chat: the host's request to send does not override a switch. Without an
# answer from the booking system the owner decides too.
ask_reasons contains "auto_reply_guests_off" if {
	point == "pre_tool_call"
	guest_message
	some r in reached
	r.guest_permissions.reachable
	holds_a_stay(r.guest_permissions)
	not r.guest_permissions.auto_reply.guests == true
}

ask_reasons contains "agent_requested_approval" if {
	point == "pre_tool_call"
	tool == "message"
	args.action == "send"
	args.requestApproval == true
}

ask_reasons contains "auto_reply_others_off" if {
	point == "pre_tool_call"
	guest_message
	some r in reached
	r.guest_permissions.reachable
	not holds_a_stay(r.guest_permissions)
	not r.guest_permissions.auto_reply.others == true
}

holds_a_stay(d) if {
	some ref in d.references
	ref.own == true
}

ask_reasons contains "pms_unreachable" if {
	point == "pre_tool_call"
	guest_message
	some r in reached
	not r.guest_permissions.reachable
}

# A message from the owner's chat to anyone else waits for the owner whatever the
# switches say: he confirms the final text the host agent wrote from his words.
# The verdict takes the first reason by name, so a more specific one shows first.
ask_reasons contains "review_owner_chat_message" if {
	point == "pre_tool_call"
	owner_turn
	guest_message
}

# Only the host switches auto-reply, and each switch waits for his 👍, so a
# model that misreads his words switches nothing.
ask_reasons contains "auto_reply_change" if {
	point == "pre_tool_call"
	owner_turn
	tool == "pms-host-set_auto_reply"
}

# Rule 5: a message to anyone but the owner that names money, a percent or an
# important word waits for the owner, from either chat. A false ask costs the
# host one tap; a miss sends money talk without him.
ask_reasons contains "important_answer" if {
	point == "pre_tool_call"
	guest_message
	some text in texts
	important_text(text)
}

important_text(text) if regex.match(money_pattern, text)

# An important word is a stem: it matches at a word start, so its inflected
# forms count (descuent, скидк).
important_text(text) if {
	some stem in pack.important_words
	regex.match(concat("", [`(?i)(^|[^\p{L}])`, stem]), text)
}

# Money in the three languages the assistant writes (Spanish, English,
# Russian): a currency sign anywhere; a number with a percent sign; a currency
# code, a currency or price word as a whole word (peso, not pesado; евро, not
# Европа; цена, not центр); a money stem at a word start; a number after
# стоит. Letter boundaries are \p{L}, because RE2's \b knows ASCII only.
money_patterns := [
	`\p{Sc}`,
	`[0-9][\s\p{Zs}]?%`,
	`[0-9][\s\p{Zs}]?р($|[^\p{L}])`,
	`(^|[^\p{L}])(mxn|usd|eur|rub|gbp|cad|dlls?|pesos?|euros?|cents?|centavos?|bucks?|money|dinero|fees?|cost(s|os?|es?)?|cuestan?|precios?|pric(e|es|ed|ing)|песо|евро|руб|цент(а|ов)?|цен(а|ы|у|ой|е))($|[^\p{L}])`,
	`(^|[^\p{L}])(d[oó]ll?ar|доллар|ro?ubl|рубл|бакс|копе[ейк]|деньг|денег|tarif|тариф|стоимост|доплат|dep[oó]sit|депозит|залог|предоплат|percent|porcentaj|por[\s\p{Zs}]+ciento|процент)`,
	`(^|[^\p{L}])сто(ит|ят|ил[аио]?)[\s\p{Zs}]+[0-9]`,
]

money_pattern := concat("", ["(?i)", concat("|", money_patterns)])

# Rule 4: no other person's data reaches a guest. Decided at the tool call on
# its arguments.
deny_reasons contains "other_persons_data" if {
	some r in reached
	r.guest_permissions.reachable
	some text in texts
	leaks(text, r.phone, r.guest_permissions)
}

# Letters and digits only, lowercase: a value written with spaces, dashes,
# dots or another case is the same value.
plain(s) := lower(regex.replace(s, `[^\p{L}\p{N}]`, ""))

digits(s) := regex.replace(s, `[^0-9]`, "")

# A reference or secret is in a text as a whole value: the text's runs of
# letters and digits are its tokens, and the value's plain form equals one or
# more consecutive tokens, whatever separates them. 7316 is in "73-16" and in
# "?code=7316", not in "17316" or in a link's "7316b52a".
mentions(text, value) if regex.match(value_pattern(value), lower(text))

value_pattern(value) := concat("", [
	`(^|[^\p{L}\p{N}])`,
	concat(`[^\p{L}\p{N}]*`, split(plain(value), "")),
	`($|[^\p{L}\p{N}])`,
])

# A Mexican mobile arrives in two forms (+52 and the older +521); both are
# the same phone. Digits only.
phone_forms(e164) := {d, concat("", ["521", substring(d, 2, 10)])} if {
	d := digits(e164)
	regex.match(`^52[0-9]{10}$`, d)
} else := {digits(e164)}

# The guest's own phone and the business's; in the owner's chat also the
# owner's, which he may give a guest.
allowed_phone_digits(guest) := phone_forms(guest) | phone_forms(normalized_phone(pack.business_phone)) if not owner_turn

allowed_phone_digits(guest) := (phone_forms(guest) | phone_forms(normalized_phone(pack.business_phone))) | phone_forms(owner_phone) if owner_turn

# A phone in a text: 10 to 15 digits (E.164's most), in groups joined by one
# or two phone separators. A longer run is no phone (a CLABE has 18 digits, a
# card 16), and a wider gap ends the run ("12.11.26 - 15.11.26" is two
# dates). A link is no phone: the payment link carries the charge's id.
phones_in(text) := {d |
	unlinked := regex.replace(text, `(?i)(https?://|www\.)\S+`, " ")
	some found in regex.find_n(`\+?[0-9](?:[ .()-]{0,2}[0-9])*`, unlinked, -1)
	d := digits(found)
	count(d) >= 10
	count(d) <= 15
}

leaks(text, guest, guest_permissions) if {
	some d in phones_in(text)
	not phone_allowed(d, guest)
	not own_reference_digits(d, guest_permissions)
}

phone_allowed(d, guest) if d in allowed_phone_digits(guest)

phone_allowed(d, guest) if {
	some allowed in allowed_phone_digits(guest)
	endswith(allowed, d)
}

# The guest's own reference written in digits alone is no phone: a
# Booking.com reservation number has 10 digits.
own_reference_digits(d, guest_permissions) if {
	some ref in guest_permissions.references
	ref.own
	digits(ref.value) == d
}

leaks(text, _, guest_permissions) if {
	some ref in guest_permissions.references
	not ref.own
	mentions(text, ref.value)
	not own_reference_covers(text, ref.value, guest_permissions)
}

# A reference that is the guest's own and contains another's is no leak.
own_reference_covers(text, value, guest_permissions) if {
	some own in guest_permissions.references
	own.own
	contains(plain(own.value), plain(value))
	mentions(text, own.value)
}

leaks(text, _, guest_permissions) if {
	some secret in guest_permissions.secrets
	not secret.allowed
	mentions(text, secret.value)
	not allowed_secret_covers(text, secret.value, guest_permissions)
}

# A secret the guest may hear that contains another's is no leak.
allowed_secret_covers(text, value, guest_permissions) if {
	some allowed in guest_permissions.secrets
	allowed.allowed
	contains(plain(allowed.value), plain(value))
	mentions(text, allowed.value)
}
