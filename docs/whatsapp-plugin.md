# Why the WhatsApp plugin differs

The demo uses `whatsapp-for-business` in place of the bundled WhatsApp plugin.
The OpenClaw application source and the Go Guardian stay unchanged.
The ACS adapter converts hooks and decisions. The WhatsApp plugin owns channel behavior.

The plugin source is `extensions/whatsapp` at the commit in [`compose.yaml`](../compose.yaml): one commit on OpenClaw `v2026.10.9`.
`git diff v2026.10.9` in the fork shows every change.

## Implemented changes

| Change | Why the demo needs it | Source in the plugin |
| --- | --- | --- |
| Separate plugin identity and configuration schema | The replacement must load on stock OpenClaw without changing the core configuration schema. | `openclaw.plugin.json`, `src/whatsapp-config-schema.ts`, `src/plugin-config.ts` |
| `requestApproval` on `message` with `action: send` | The agent can request human review even when automatic replies are enabled. The policy decides the request. | `src/channel-actions.ts` |
| Approval authorization separate from inbox access | Guests may write to an open inbox. Inbox access must not authorize approval. | `src/exec-approvals.ts`, `src/approval-native.ts`, `src/approval-reactions.ts` |
| Approval routing and `notifyOrigin` | Cards go to authorized approvers. The demo can suppress approval notices in the guest conversation. | `src/exec-approvals.ts`, `src/approval-native.ts` |
| Message cards in English or Spanish | The host sees the proposed recipient, text, other arguments and available conversation context. A reaction or another non-send action is named on the card. | `src/approval-card.ts`, `src/approval-handler.runtime.ts` |
| History written by the send action | A host agent's send to a guest enters the guest's transcript, so the guest agent and the card see that the guest has a reply. OpenClaw's `message_sent` hook did not reach the plugin on that path. | `src/channel-send-action.ts`, `src/conversation-history.ts` |
| Cards in the approver's history | The host answers a card with a short message ("give her a special price: $100"). The card and its outcome enter the host agent's transcript, with the guest's session, so the host agent knows which guest and which question the message of the host is about. Without it the host agent asked whom the host meant. | `src/approval-handler.runtime.ts`, `src/conversation-history.ts` |
| Guest conversation history | Manual replies and old guest messages enter the guest transcript. | `src/conversation-history.ts`, `src/inbound/message-delivery.ts` |
| Channel-owned `send` | Delivery hooks retain the requesting agent's session when the host sends to a guest. The native delivery mechanism enforces policy and cancellation. | `src/channel-send-action.ts`, `src/channel.ts` |
| Unanswered messages of the recipient | A host-initiated proposal shows guest questions. Replies to other recipients do not clear those questions. | `src/approval-card.ts` |
| Reactions on every part of a long card | WhatsApp can split a card into several messages. Each part must identify the same approval. | `src/approval-handler.runtime.ts` |
| Outcome in the card itself | The host sees that a card is closed: the plugin edits its reaction line into the outcome, so a reaction left on an old card does not look like a pending decision. | `src/approval-handler.runtime.ts`, `src/inbound/send-api.ts` |
| Stale cards refused on a business reply or a newer card | A host's reply or the host agent's newer card makes the guest agent's open proposal outdated; a 👍 on it would send a second, outdated reply. | `src/approval-stale-cards.ts`, `src/conversation-history.ts` |
| Refusal-only card when the proposed call cannot be read | The host must not approve a message whose contents are unavailable. | `src/approval-handler.runtime.ts` |
| The `owner` recipient | An agent can contact the host (`commands.ownerAllowFrom`) without inventing or keeping the phone number of the host. | `src/command-owner.ts`, `src/resolve-outbound-target.ts` |
| One identity for Mexican `+52` and `+521` mobile forms | Incoming messages, configured people and outgoing targets must identify the same person. | `src/normalize-target.ts`, `src/targets-runtime.ts` |
| Lookup of the registered WhatsApp chat before sending | Written phone digits can differ from the account's registered chat identifier. A failed lookup fails the send. | `src/inbound/monitor.ts`, `src/inbound/send-api.ts` |

These are the behavioral changes and the packaging needed to load them.
The plugin also includes tests, its license and its notice file.
The demo configures approvals through `commands.ownerAllowFrom`; the plugin also supports an explicit `execApprovals.approvers` list.

## Conversation history

OpenClaw's transcript is the only history store.
The plugin records messages typed on the business phone without starting an agent turn.
Old guest messages delivered after reconnect enter history after normal access checks, without automatic replies.
Text, captions, locations and attachment markers remain visible. Markers do not contain the attachment's contents.

Set `plugins.entries.whatsapp-for-business.config.approvalUnansweredMessageLimit` in the [configuration template](../openclaw/config/openclaw.template.json).
The default and maximum are 10; the minimum is 1.
The approval card shows the recipient's latest unanswered messages in sent-time order.
A delivered business reply ends that sequence. A failed send or a reply to another recipient does not.
A manual reply from the business phone to a guest refuses the open cards for that guest, recorded as a refusal by the host; the card shows "already answered" (↪️). A card of the host agent for that guest refuses them when it appears; they show "updated below" (🔄).

The native incoming queue retries failed transcript writes. Message identities prevent repeated observations from creating repeated entries.
The plugin uses OpenClaw's exported `session-transcript-runtime` writer. OpenClaw classifies this SDK path as private-local.
Test the packaged plugin before changing the OpenClaw version.

The plugin's `send` action writes a cross-agent send into the recipient's transcript after WhatsApp accepts it.
It does not rely on OpenClaw's `message_sent` hook: in a live run that hook never reached the plugin for a host agent's send to a guest.
The write follows delivery, so a process failure between them loses that history entry; a failed write is logged as `whatsapp/history` and the send still counts as delivered.
A media-only send records an attachment marker without the file name.

The plugin records only the messages that WhatsApp delivers to the linked device. So the history can miss messages of the account.

These requirements do not justify rental policy inside the plugin.
Reservation ownership, guest permissions and sensitive-topic rules remain in the demo's ACS policy.
