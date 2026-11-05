"""Refuse an OpenClaw configuration that breaks a guarantee of the demo.

The ACS adapter only converts hooks; it checks no OpenClaw setting. The
settings it depends on are checked here: by setup.sh before the stack starts,
and by `make test-stack` on the running stack. README.md, "OpenClaw settings",
says what breaks without each one.

Usage: check-config.py <openclaw.json> <guardian.json> [<plugins inspect acs-guardian-client --runtime --json>]
"""

import json
import sys

ADAPTER = "acs-guardian-client"
WHATSAPP = "whatsapp-for-business"
# The hooks through which every run, tool call and delivery reaches the Guardian.
ADAPTER_HOOKS = ("before_agent_run", "before_tool_call", "message_sending")


def problems(config, guardian, adapter_runtime):
    out = []

    def need(ok, text):
        if not ok:
            out.append(text)

    need(config.get("messages", {}).get("visibleReplies") == "message_tool",
         "messages.visibleReplies must be message_tool: every reply is a message call, which the Guardian decides")
    queue = config.get("messages", {}).get("queue", {})
    need(queue.get("byChannel", {}).get("whatsapp", queue.get("mode")) == "interrupt",
         "messages.queue.byChannel.whatsapp must be interrupt: a new message closes the open card, and the next card answers every message")
    need(config.get("channels", {}).get("whatsapp", {}).get("groupPolicy") == "disabled",
         "channels.whatsapp.groupPolicy must be disabled: the rules know one sender per chat")
    allow_from = config.get("commands", {}).get("allowFrom", {})
    need(all("*" not in v for v in allow_from.values()),
         "commands.allowFrom must name operators, never *: a command can change the runtime without a Guardian step")

    defaults = config.get("agents", {}).get("defaults", {})
    models = defaults.get("models", {})
    used = {defaults.get("model", {}).get("primary"), defaults.get("utilityModel")}
    used |= {e.get("model", {}).get("primary") for e in config.get("agents", {}).get("entries", {}).values()}
    for model in sorted(m for m in used if m):
        need(models.get(model, {}).get("agentRuntime", {}).get("id") == "openclaw",
             f"agents.defaults.models[{model!r}].agentRuntime.id must be openclaw: other runtimes skip hooks the adapter needs")

    plugin_cfg = config.get("plugins", {})
    need(ADAPTER in plugin_cfg.get("allow", []), f"plugins.allow must list {ADAPTER}")
    entry = plugin_cfg.get("entries", {}).get(ADAPTER, {})
    need(entry.get("enabled") is True, f"plugins.entries.{ADAPTER}.enabled must be true")
    hooks = entry.get("hooks", {})
    need(hooks.get("allowConversationAccess") is True,
         f"plugins.entries.{ADAPTER}.hooks.allowConversationAccess must be true: without it OpenClaw withholds the run hooks")
    need("timeoutMs" not in hooks and "timeouts" not in hooks,
         f"plugins.entries.{ADAPTER}.hooks must set no timeoutMs or timeouts: the adapter registers its hooks with a timeout above its deadline, and a configured one replaces it")

    wa = plugin_cfg.get("entries", {}).get(WHATSAPP, {}).get("config", {})
    owners = [o for o in config.get("commands", {}).get("ownerAllowFrom", []) if o.startswith("whatsapp:")]
    need(len(owners) == 1,
         "commands.ownerAllowFrom must name exactly one WhatsApp owner: the approver of every card and the `owner` target")
    need("approvers" not in wa.get("execApprovals", {}),
         f"plugins.entries.{WHATSAPP}.config.execApprovals.approvers must be unset: the owner in commands.ownerAllowFrom is the only approver")
    need(wa.get("execApprovals", {}).get("notifyOrigin") is False,
         f"plugins.entries.{WHATSAPP}.config.execApprovals.notifyOrigin must be false: otherwise the guest is told that an approval went to the host")
    need(wa.get("approvalLanguage") == "es", f"plugins.entries.{WHATSAPP}.config.approvalLanguage must be es: the host reads Spanish cards")
    need(config.get("approvals", {}).get("plugin", {}).get("enabled") is not True,
         "approvals.plugin must stay off: a forwarded approval shows OpenClaw's own text, not the proposed message, and still takes 👍")
    wa_channel = config.get("channels", {}).get("whatsapp", {})
    prefixed = ["messages"] if "responsePrefix" in config.get("messages", {}) else []
    prefixed += ["channels.whatsapp"] if "responsePrefix" in wa_channel else []
    prefixed += [f"channels.whatsapp.accounts.{a}" for a, v in wa_channel.get("accounts", {}).items()
                 if "responsePrefix" in v]
    need(not prefixed,
         f"responsePrefix must be unset ({', '.join(prefixed)}): OpenClaw adds it after the host approved the text on the card")

    search = config.get("tools", {}).get("web", {}).get("search", {})
    need(search.get("enabled") is False or search.get("provider") not in (None, "", "auto", "openai"),
         "tools.web.search.provider must name a plugin provider such as parallel-free: an unset or openai provider lets an OpenAI model search inside its own model call, which no Guardian step sees")

    need(guardian.get("failurePosture") == "closed",
         "guardian.json failurePosture must be closed: no step runs without a decision")

    if adapter_runtime is not None:
        hooks = {h["name"] for h in adapter_runtime.get("typedHooks", [])}
        for hook in ADAPTER_HOOKS:
            need(hook in hooks, f"plugin {ADAPTER} loads without its {hook} hook: OpenClaw would run steps no Guardian decides")
    return out


def main(argv):
    if len(argv) not in (3, 4):
        sys.exit(__doc__)
    config = json.load(open(argv[1]))
    guardian = json.load(open(argv[2]))
    adapter_runtime = json.load(open(argv[3])) if len(argv) == 4 else None
    found = problems(config, guardian, adapter_runtime)
    for p in found:
        print(f"OpenClaw configuration: {p}", file=sys.stderr)
    sys.exit(1 if found else 0)


if __name__ == "__main__":
    main(sys.argv)
