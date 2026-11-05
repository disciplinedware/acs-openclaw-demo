#!/usr/bin/env bash
# Checks .env and fills its empty generated values (PMS_API_KEY,
# OPENCLAW_GATEWAY_TOKEN), and writes the Guardian's HMAC key to
# .secrets/acs-hmac once. Needs bash and coreutils.
set -euo pipefail
{
cd "$(dirname "${BASH_SOURCE[0]}")/.."
[[ -f .env ]] || { echo "No .env: run 'cp .env.example .env' and fill BUSINESS_PHONE, HOST_PHONE and a model key." >&2; exit 1; }

hex32() { od -An -tx1 -N32 /dev/urandom | tr -d ' \n'; }
base64_32() { head -c 32 /dev/urandom | base64 | tr -d '\n'; }

fill() {
	local name="$1" value
	if grep -qE "^${name}=.+" .env; then
		return
	fi
	value="$($2)"
	if grep -qE "^${name}=" .env; then
		awk -v n="$name" -v v="$value" 'BEGIN { FS = OFS = "=" } $1 == n { print n "=" v; next } { print }' .env > .env.tmp
		mv .env.tmp .env
	else
		printf '%s=%s\n' "$name" "$value" >> .env
	fi
	echo "generated $name in .env"
}
umask 077
fill PMS_API_KEY hex32
fill OPENCLAW_GATEWAY_TOKEN hex32
# The folder keeps other users of this machine out; the file is readable so
# the Guardian's and OpenClaw's container users can read it.
mkdir -p .secrets
if [[ ! -s .secrets/acs-hmac ]]; then
	base64_32 > .secrets/acs-hmac
	echo "generated .secrets/acs-hmac"
fi
# Set on every run, so a restored or copied .env and .secrets get them too.
chmod 600 .env
chmod 700 .secrets
chmod 644 .secrets/acs-hmac

for name in BUSINESS_PHONE HOST_PHONE; do
	grep -qE "^${name}=\+[1-9][0-9]{6,14}$" .env || { echo "$name in .env is not an E.164 number such as +447700900002" >&2; exit 1; }
done
if grep -q '^GUARDIAN=' .env; then
	grep -qE '^GUARDIAN=(ts|go)$' .env || { echo "GUARDIAN in .env is ts or go" >&2; exit 1; }
fi
# Each model's provider needs its own key: an Anthropic key does not run an openai/ model.
for name in AGENT_MODEL HOST_AGENT_MODEL; do
	model="$(sed -n "s/^${name}=//p" .env | tail -n 1)"
	case "$model" in
		anthropic/*) key=ANTHROPIC_API_KEY ;;
		openai/*) key=OPENAI_API_KEY ;;
		*) echo "$name in .env is '$model': use anthropic/<model> or openai/<model>" >&2; exit 1 ;;
	esac
	grep -qE "^${key}=.+" .env || { echo "$name is $model: set $key in .env, or pick a model of the provider whose key you set" >&2; exit 1; }
done
exit
}
