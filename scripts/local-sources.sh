#!/usr/bin/env bash
# Writes tmp/local-sources.yaml, which the Makefile adds to every compose command.
# make up: empty, so every source comes from GitHub at the commits compose.yaml and guardian/compose.*.yaml pin.
# make local-up (--working-tree): the working files of OPENCLAW_REPO and ACS_REPO in .env.
set -euo pipefail
{
cd "$(dirname "${BASH_SOURCE[0]}")/.."
override=tmp/local-sources.yaml
mkdir -p tmp
case "${1:-}" in
	"")
		echo "services: {}" > "$override"
		exit 0
		;;
	--working-tree) ;;
	*) echo "Usage: local-sources.sh [--working-tree]" >&2; exit 1 ;;
esac

setting() { sed -n "s/^$1=//p" .env 2>/dev/null | tail -n 1; }
folder() {
	[[ -d "$2" ]] || { echo "$1 in .env is not a folder: $2" >&2; exit 1; }
	(cd "$2" && pwd)
}
openclaw_repo="$(setting OPENCLAW_REPO)"
acs_repo="$(setting ACS_REPO)"
[[ -n "$openclaw_repo" ]] || { echo "make local-up requires OPENCLAW_REPO in .env: the WhatsApp plugin's OpenClaw checkout." >&2; exit 1; }
plugin_src="$(folder OPENCLAW_REPO "$openclaw_repo")"
[[ -z "$acs_repo" ]] || acs_src="$(folder ACS_REPO "$acs_repo")"
{
	echo "# Written by scripts/local-sources.sh from OPENCLAW_REPO and ACS_REPO in .env."
	echo "services:"
	printf '  openclaw:\n    build:\n      additional_contexts:\n'
	printf '        whatsapp-plugin-src: %s\n' "$plugin_src"
	[[ -z "$acs_repo" ]] || printf '        acs: %s\n' "$acs_src"
} > "$override.tmp"
mv "$override.tmp" "$override"
exit
}
