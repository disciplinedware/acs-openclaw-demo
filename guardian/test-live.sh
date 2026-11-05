#!/usr/bin/env bash
# The talk's scenarios on the real rules against a running fake PMS
# (guardian/test-live/talk_test.rego). Needs Docker only.
set -euo pipefail
{
cd "$(dirname "${BASH_SOURCE[0]}")/.."
compose=(docker compose -p acs-openclaw-demo-test -f guardian/compose.test.yaml)
trap '"${compose[@]}" down --remove-orphans >/dev/null 2>&1' EXIT
mkdir -p tmp/guardian-test
"${compose[@]}" run --rm -T --no-deps opa eval --format raw -d guardian/cases/cases.yaml \
	'json.marshal({"cases": data})' > tmp/guardian-test/live-data.json
mkdir -p tmp/guardian-test/bookings
# The test bookings, moved so that CC-1042 arrives today, as make bookings-move does.
image="$(docker build -q fake-pms)"
docker run --rm --network none --read-only --cap-drop ALL --user "$(id -u):$(id -g)" \
	-v "$PWD/fake-pms/testdata:/in:ro" -v "$PWD/tmp/guardian-test/bookings:/data" "$image" \
	bookings-move -file /in/bookings.json -out /data/bookings.json -ref CC-1042
"${compose[@]}" up -d --build --wait fake-pms
"${compose[@]}" run --rm -T opa-live test --verbose guardian/policy/lib guardian/test-live tmp/guardian-test/live-data.json
exit
}
