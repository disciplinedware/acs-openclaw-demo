# Host and guest workflow

## Purpose

The project demonstrates a vacation-rental assistant with ACS controls on its tools and replies.
The host gets a private assistant.
Guests and strangers write to the guest agent. A guest can get information only about their own reservation.
The two agents run in one OpenClaw instance on one WhatsApp business account.
They use separate conversations, not separate WhatsApp accounts.

## Component boundaries

- OpenClaw provides the agent runtime, conversations, tools and scheduling.
- The standard ACS OpenClaw adapter converts OpenClaw hooks into ACS steps and applies the Guardian's decisions.
- The published Go Guardian evaluates the demo's policy. The demo does not change the Guardian.
- The WhatsApp plugin provides the channel and its approval cards.
- The fake PMS gives reservation information from [`bookings.json`](../fake-pms/data/bookings.json). PMS means property management system.
- The [Rego policy](../guardian/policy/lib/assistant.rego) owns the rental-specific restrictions.

Both `make up` and `make local-up` use the official OpenClaw 2026.10.9 image pinned by digest in [`compose.yaml`](../compose.yaml).
The demo installs the WhatsApp plugin, the ACS adapter and OpenClaw's web search plugin `@openclaw/parallel-plugin` on that image. It does not compile the OpenClaw application.
The image also replaces selected bundled dependencies through [`replace-bundled-packages.sh`](../openclaw/replace-bundled-packages.sh).
The resulting image is therefore not the unmodified upstream image.
Setup gives the OpenClaw process read access to its configuration through owner-only permissions. The runtime mounts configuration read-only.

The [WhatsApp plugin changes](whatsapp-plugin.md) document each extension and its reason.
The [README settings table](../README.md#openclaw-settings) lists the required configuration.

## Host conversation

The host writes from `HOST_PHONE` to `BUSINESS_PHONE`.
The binding selects the `host` agent.
The host can discuss guest requests, read guest conversations and keep notes in the writable `memory/` directory.
Messages to the host do not require routine guest-reply approval.
The host sees WhatsApp's typing indicator while the host agent works (`typingMode: instant` on the `host` agent); guests never see it (`agents.defaults.typingMode: never`).

The demo does not synchronize with Booking.com or Airbnb. [What production needs](production.md) tells how a product adds it.
As shipped, the host agent has `exec` (`OPENCLAW_EXEC_TOOL=on`), and the supplied policy allows it for the host only. With `off`, no agent has it.
The tool configuration does not offer a browser to the host.
The host agent can search the web with `web_search`. The tool uses Parallel Search Free and needs no key. Each call goes through the Guardian, which allows web search only for the host. Parallel receives the search queries of the host.
No agent can fetch a web page: OpenClaw has `web_fetch` off, and the Guardian refuses it. A fetch sends a URL to any server. So the text of a guest could make the host agent put private data in a URL, or reach the internal services of the stack.
OpenClaw cannot write the reservations file.
The fake PMS has six read tools and no tool that writes a reservation.
The supplied host instructions also prohibit scheduled messages.
Do not give the host agent the credentials of a booking platform: the demo cannot use them.

## Guest conversation

1. A guest writes to `BUSINESS_PHONE`.
2. OpenClaw selects the `assistant` agent and a conversation scoped to that sender.
3. The agent gets reservation information through the MCP server of the fake PMS.
4. The Guardian compares each protected phone argument with the authenticated channel sender.
5. The fake PMS returns the door code and the Wi-Fi password of a home only while a paid reservation of that home is in its access window. It does not check who asks. The Guardian checks the result of each reservation lookup and access lookup before the model reads it. The Guardian also refuses a reply that contains a door code or a Wi-Fi password that this guest may not have.
6. The agent proposes its reply through the `message` tool.
7. The Guardian allows, refuses or requests host approval for that call.

The guest cannot use host memory, files, automation, conversation-history or web search tools.
The guest agent never has `exec`. The host agent has it while `OPENCLAW_EXEC_TOOL=on`.
The public home guides and the extras catalog are available to strangers too.
The Guardian passes the data of a reservation and the access data of a home only to the guest of that reservation.

## Replies and approval

In the shipped reservations file, the `guests` switch is `true` and the `others` switch is `false`.
So each reply to a stranger needs the approval of the host.
The `guests` switch applies to guests: people with a current or future reservation.
The `others` switch applies to strangers.
These switches are JSON fields, not a graphical checkbox or a supplied host tool.

With the relevant switch enabled, an ordinary reply of the guest agent can proceed automatically.
A matching money expression or sensitive keyword still requires approval.
The agent can also set `requestApproval: true`.
A policy refusal takes priority over approval.
When the PMS guest permissions read fails, the policy requests host approval instead of automatic delivery.

The host reads the proposed reply and approves with 👍 or refuses with 👎.
Approval applies to the pending proposal.
When the card ends, the plugin edits the card itself: its last line shows the outcome: approved (✅), refused (❌), expired (⏱️), already answered (↪️) or updated below (🔄). The cards are in Spanish (`approvalLanguage`: `es` in [`openclaw.template.json`](../openclaw/config/openclaw.template.json)).
A new guest message interrupts the current proposal under the configured WhatsApp queue policy.

The host can also ask the host agent to write to a guest.
The host agent reads the guest conversation back to the last delivered reply, with tool results, and checks the related reservation facts.
It replies to each open question that it has facts for. When the host tells it a fact that answers an open question of the guest, it proposes that reply to the guest.
Every message the host agent writes to a guest goes to the host on a card, whatever the automatic-reply switches say: the host confirms the final text.
The sensitive-content rules apply to it too, and a host instruction to send does not bypass them.
The WhatsApp plugin keeps the host's admitted session for delivery checks when the host sends to a guest.
The guest does not need a prior admitted run for that host-requested send.
The WhatsApp plugin reads the recipient's unanswered messages for the approval card, including a reply requested by the host.
Set `approvalUnansweredMessageLimit` in [`openclaw.template.json`](../openclaw/config/openclaw.template.json) to an integer from 1 to 10. The default is 10.
The card counts earlier unanswered messages when the limit hides them.
Only a delivered reply to that guest ends the unanswered sequence. Failed sends and messages to other recipients do not.

## Direct conversation between host and guest

The host can type directly to a guest from the business WhatsApp account.
Either person can send the first message.
The guest agent sees both sides of that conversation, so that it does not ask the same question again or contradict the reply of the host.
Its history marks these messages `Business phone:` or `Business agent:`. Its instructions do not say that such a message outranks the data of the PMS: the model weighs the history on its own.

The WhatsApp plugin records manual messages in the guest's OpenClaw transcript without starting an agent turn.
Old guest messages received after reconnect also enter history after the normal access check. They do not start automatic replies.
The plugin uses sent timestamps, so an old reply does not hide newer unanswered questions.
A manual reply to a guest refuses the guest agent's open cards for that guest: an outdated proposal cannot reach the guest after the host answered. The card shows "already answered" (↪️). A card of the host agent for that guest refuses them as soon as it appears, and they show "updated below" (🔄), so one proposal stays open.
See [the plugin's history boundary](whatsapp-plugin.md#conversation-history) for delivery and attachment limits.

## Security boundary

The supplied Compose configuration publishes no host ports.
Guests communicate through WhatsApp; they do not receive Gateway, Guardian or MCP credentials.
An empty `ALLOW_FROM` admits all WhatsApp senders.
Set `ALLOW_FROM` to the demonstration participants for a controlled rehearsal.

ACS enforces the rules that this deployment declares.
The model, OpenClaw and WhatsApp keep their own security models.
The text rules detect specified patterns, not every sensitive meaning or every encoding of private data.
The host agent can read guest content, so malicious guest instructions can also reach that agent indirectly.
Host approval remains a human decision, not proof that the proposed reply is safe.

The demo uses invented reservations.
A successful test run is not approval to expose the Gateway to the internet or connect real guest data and platform credentials.
See [OpenClaw's security model](https://docs.openclaw.ai/gateway/security) before extending the deployment's trust boundary.
[What production needs](production.md) lists the limits of this demo.
