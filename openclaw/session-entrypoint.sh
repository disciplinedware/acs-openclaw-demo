#!/usr/bin/env bash
set -euo pipefail
{

exec 9>"${OPENCLAW_STATE_DIR:-/home/node/.openclaw}/.session.lock"
flock --exclusive --nonblock 9 || {
	echo "OpenClaw state is already in use by another process" >&2
	exit 1
}

"$@" &
gateway=$!
trap 'kill -TERM "$gateway" 2>/dev/null || true' TERM
trap 'kill -INT "$gateway" 2>/dev/null || true' INT

# Keep the lock when a signal interrupts wait or Node replaces its process.
while true; do
	status=0
	wait "$gateway" || status=$?
	kill -0 "$gateway" 2>/dev/null || exit "$status"
done
exit
}
