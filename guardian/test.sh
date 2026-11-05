#!/usr/bin/env bash
# Unit tests of the host assistant's rules in the OPA version the Guardian
# pins: the shared case set and the phone vectors. Needs Docker only (or
# OPA_BIN). On a failure it prints each case that decided otherwise.
set -euo pipefail
{
cd "$(dirname "${BASH_SOURCE[0]}")/.."
mkdir -p tmp/guardian-test
opa() {
	if [[ -n "${OPA_BIN:-}" ]]; then
		"$OPA_BIN" "$@"
		return
	fi
	docker compose -f guardian/compose.test.yaml run --rm -T --no-deps opa "$@"
}
# Both YAML files load at the data root; the tests read them as data.cases
# and data.phone_vectors.
opa eval --format raw -d guardian/cases/cases.yaml -d guardian/testdata/phones.yaml \
	'json.marshal({"cases": object.remove(data, ["vectors"]), "phone_vectors": data.vectors})' \
	> tmp/guardian-test/data.json
if ! opa test guardian/policy/lib guardian/test tmp/guardian-test/data.json; then
	opa eval --format pretty -d guardian/policy/lib -d guardian/test -d tmp/guardian-test/data.json 'data.assistant_test.failures' >&2
	exit 1
fi
exit
}
