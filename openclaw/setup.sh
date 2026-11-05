#!/usr/bin/env bash
# Renders the demo's setup from .env, run by the `setup` service before the
# Guardian and OpenClaw start:
#   /setup/openclaw   OpenClaw's configuration and the adapter's Guardian
#                     settings (guardian.json)
#   /setup/guardian   the Go Guardian's deployment, with the policy data of
#                     this host (policy/lib/data.json), for the Guardian's
#                     user alone: it holds the booking system's key
#
#   HOST_PHONE        the host's personal number, the agent's only owner (E.164)
#   BUSINESS_PHONE    the business number linked to OpenClaw (E.164)
#   ALLOW_FROM        who may write to the business number: E.164 numbers,
#                     comma-separated; empty admits everyone
#   AGENT_MODEL       the guest agent's model, provider/model
#   HOST_AGENT_MODEL  the host agent's model, provider/model
#   OPENCLAW_ACS_ADAPTER off: the ACS adapter stays off and no Guardian decides
#                       anything, to compare the demo without ACS
#   OPENCLAW_EXEC_TOOL  on: the host agent has OpenClaw's exec tool; off: no agent has it
#   PMS_URL, PMS_API_KEY  the fake PMS as the Guardian reaches it
#   GUARDIAN_UID      the user the Guardian runs as
#
# /setup/memory is the host agent's memory volume, handed to OpenClaw's user.
set -euo pipefail
{
src=/opt/demo
out=/setup
personal="${HOST_PHONE:?HOST_PHONE is required (.env)}"
business="${BUSINESS_PHONE:?BUSINESS_PHONE is required (.env)}"
pms_url="${PMS_URL:?PMS_URL is required}"
key="${PMS_API_KEY:?PMS_API_KEY is required (make up generates it)}"
guardian_uid="${GUARDIAN_UID:?GUARDIAN_UID is required}"
exec_tool="${OPENCLAW_EXEC_TOOL:?OPENCLAW_EXEC_TOOL is required}"
acs_adapter="${OPENCLAW_ACS_ADAPTER:?OPENCLAW_ACS_ADAPTER is required}"
model="${AGENT_MODEL:?AGENT_MODEL is required (.env)}"
host_model="${HOST_AGENT_MODEL:?HOST_AGENT_MODEL is required (.env)}"
model_ref='^[a-z0-9-]+/[A-Za-z0-9._-]+$'
[[ "$model" =~ $model_ref ]] || { echo "AGENT_MODEL is not a provider/model reference: $model" >&2; exit 1; }
[[ "$host_model" =~ $model_ref ]] || { echo "HOST_AGENT_MODEL is not a provider/model reference: $host_model" >&2; exit 1; }
case "$exec_tool" in
	on | off) ;;
	*) echo "OPENCLAW_EXEC_TOOL must be on or off: $exec_tool" >&2; exit 1 ;;
esac
case "$acs_adapter" in
	on | off) ;;
	*) echo "OPENCLAW_ACS_ADAPTER must be on or off: $acs_adapter" >&2; exit 1 ;;
esac
e164='^\+[1-9][0-9]{6,14}$'
[[ "$personal" =~ $e164 ]] || { echo "HOST_PHONE is not an E.164 number (+ and digits)" >&2; exit 1; }
[[ "$business" =~ $e164 ]] || { echo "BUSINESS_PHONE is not an E.164 number (+ and digits)" >&2; exit 1; }
# One number per Mexican mobile, as the WhatsApp plugin gives an inbound sender: +521 and ten
# digits is the older form of +52 and ten digits. OpenClaw matches bindings and owners by this text.
one_number() { sed -E 's/^\+521([0-9]{10})$/+52\1/' <<< "$1"; }
personal="$(one_number "$personal")"
business="$(one_number "$business")"
[[ "$business" != "$personal" ]] || { echo "BUSINESS_PHONE cannot be the host's personal number" >&2; exit 1; }

policy=open
allow='["*"]'
if [[ -n "${ALLOW_FROM:-}" ]]; then
	policy=allowlist
	allow='['
	IFS=, read -ra numbers <<< "$ALLOW_FROM"
	for n in "${numbers[@]}"; do
		n="${n// /}"
		[[ -n "$n" ]] || continue
		[[ "$n" =~ $e164 ]] || { echo "ALLOW_FROM holds a number that is not E.164 (+ and digits)" >&2; exit 1; }
		allow+="\"$(one_number "$n")\","
	done
	allow="${allow%,}]"
fi

# Rendered into a staging folder, then copied over $out in place: a running
# container reads these files, and a replaced inode would leave it reading
# the deleted ones.
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/openclaw" "$stage/guardian"
sed -e "s|@PERSONAL_PHONE@|$personal|g" -e "s|@DM_POLICY@|$policy|g" -e "s|@ALLOW_FROM@|$allow|g" \
	-e "s|@WHATSAPP_ENABLED@|true|g" -e "s|@AGENT_MODEL@|$model|g" \
	"$src/openclaw/config/openclaw.template.json" > "$stage/openclaw/openclaw.json"
python3 - "$stage/openclaw/openclaw.json" "$host_model" "$exec_tool" "$acs_adapter" <<'PY'
import json, sys
path, host_model, exec_tool, acs_adapter = sys.argv[1:]
config = json.load(open(path))
defaults = config["agents"]["defaults"]
# The host agent runs its own model, which the model policy and the runtime map also name.
config["agents"]["entries"]["host"]["model"] = {"primary": host_model}
if host_model not in defaults["modelPolicy"]["allow"]:
    defaults["modelPolicy"]["allow"].append(host_model)
defaults["models"].setdefault(host_model, {"agentRuntime": {"id": "openclaw"}})
if exec_tool == "on":
    config["tools"]["deny"].remove("group:runtime")
    config["tools"]["allow"].append("exec")
if acs_adapter == "off":
    config["plugins"]["entries"]["acs-guardian-client"]["enabled"] = False
with open(path, "w") as f:
    json.dump(config, f, indent=2, ensure_ascii=False)
PY
cp "$src/openclaw/config/guardian.json" "$stage/openclaw/guardian.json"
# Every check keeps a step from passing the Guardian; without the adapter no step meets one.
if [[ "$acs_adapter" == on ]]; then
	python3 "$src/check-config.py" "$stage/openclaw/openclaw.json" "$stage/openclaw/guardian.json"
fi
chmod 755 "$stage/openclaw"
chown node:node "$stage/openclaw/openclaw.json" "$stage/openclaw/guardian.json"
chmod 600 "$stage/openclaw/openclaw.json" "$stage/openclaw/guardian.json"

cp -R "$src/guardian/guardian.yaml" "$src/guardian/mapping.yaml" "$src/guardian/policy" "$stage/guardian/"
python3 - "$src/guardian/pack.yaml" "$stage/guardian/policy/lib/data.json" "$personal" "$business" "$pms_url" "$key" <<'PY'
import json, sys, yaml
settings, target, personal, business, pms_url, key = sys.argv[1:]
pack = yaml.safe_load(open(settings))
pack.update(personal_phone=personal, business_phone=business, pms_url=pms_url, connect_key=key)
with open(target, "w") as f:
    json.dump({"pack": pack}, f)
PY
chown -R "$guardian_uid:$guardian_uid" "$stage/guardian"
chmod -R u=rwX,go= "$stage/guardian"

rsync -a --delete --inplace --checksum "$stage/openclaw/" "$out/openclaw/"
rsync -a --delete --inplace --checksum "$stage/guardian/" "$out/guardian/"
chmod 755 "$out/openclaw"
chown node:node /setup/memory
echo "setup rendered: models $model / $host_model, exec tool $exec_tool, ACS adapter $acs_adapter"
exit
}
