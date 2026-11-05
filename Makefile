.DEFAULT_GOAL := help
# The Guardian: go (PR #169) or ts (the TypeScript reference, PR #229), from .env.
GUARDIAN := $(or $(shell sed -n 's/^GUARDIAN=//p' .env 2>/dev/null),go)
# off runs OpenClaw with no Guardian, to compare the demo without ACS (README.md, Compare it without ACS).
ACS_ADAPTER := $(or $(shell sed -n 's/^OPENCLAW_ACS_ADAPTER=//p' .env 2>/dev/null),on)
# tmp/local-sources.yaml (scripts/local-sources.sh): empty after make up, the local working files after make local-up.
COMPOSE := docker compose -f compose.yaml -f guardian/compose.$(GUARDIAN).yaml $$(test -f tmp/local-sources.yaml && echo -f tmp/local-sources.yaml)

help: ## List the commands
	@grep -hE '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN { FS = ":.*## " } { printf "  make %-14s %s\n", $$1, $$2 }'

up: ## Build the demo from GitHub at the pinned commits, on the pinned official OpenClaw image, and start the stack
	@./scripts/env.sh
	@./scripts/local-sources.sh
	@$(MAKE) --no-print-directory start

# See README.md, Change it: published pins stay in make up.
local-up: ## Build local plugin and adapter changes and start the stack
	@./scripts/env.sh
	@./scripts/local-sources.sh --working-tree
	@$(MAKE) --no-print-directory start

start:
	$(COMPOSE) up -d --build --wait --wait-timeout 1200
	@# The Guardian and OpenClaw read the setup only when they start; setup re-renders it on every up.
	$(COMPOSE) restart acs-guardian openclaw
	$(COMPOSE) up -d --no-build --wait --wait-timeout 1200 acs-guardian openclaw
	@$(MAKE) --no-print-directory require-adapter

# Without the ACS adapter nothing decides OpenClaw's replies: stop OpenClaw rather than run it ungoverned.
require-adapter:
ifeq ($(ACS_ADAPTER),off)
	@echo "WARNING: OPENCLAW_ACS_ADAPTER=off: no Guardian decides anything, and every reply reaches the guest at once." >&2
else
	@$(MAKE) --no-print-directory test-stack test-guardian || { $(COMPOSE) stop openclaw; echo "OpenClaw is stopped: the checks above failed." >&2; exit 1; }
endif

down: ## Stop the stack; the WhatsApp link and the audit logs stay in their volumes
	$(COMPOSE) down

restart: ## Restart OpenClaw after an edit of the agents' instructions
	$(COMPOSE) restart openclaw

whatsapp-link: ## Link the business number: scan the printed QR from the business phone
	$(COMPOSE) exec openclaw python3 /opt/demo/whatsapp-link.py

WHATSAPP_FROM ?= .secrets/whatsapp
whatsapp-import: ## Reuse a linked WhatsApp session instead of a QR: copy WHATSAPP_FROM (default .secrets/whatsapp) into OpenClaw
	@test -s "$(WHATSAPP_FROM)/creds.json" || { echo "No WhatsApp session in $(WHATSAPP_FROM): it needs creds.json, the folder credentials/whatsapp/default of an OpenClaw state" >&2; exit 1; }
	$(COMPOSE) stop openclaw
	$(COMPOSE) run --rm --no-deps -T -v "$(abspath $(WHATSAPP_FROM)):/import:ro" --entrypoint sh openclaw -c \
		'rm -rf /home/node/.openclaw/credentials/whatsapp/default && mkdir -p /home/node/.openclaw/credentials/whatsapp && cp -R /import /home/node/.openclaw/credentials/whatsapp/default'
	$(COMPOSE) up -d --wait --wait-timeout 1200 openclaw
	@$(MAKE) --no-print-directory require-adapter

DAY ?=
# Demonstration data only: the rules and the fake PMS take any dates, and a real deployment writes bookings.json from its booking sync instead.
bookings-move: ## Move the demonstration bookings so that the stage guest's CC-1042 arrives on DAY=YYYY-MM-DD (default: today in Cancún)
	@image="$$(docker build -q fake-pms)" && docker run --rm --network none --read-only --cap-drop ALL --user "$$(id -u):$$(id -g)" \
		-v "$(CURDIR)/fake-pms/data:/data" "$$image" bookings-move -ref CC-1042 -day "$(DAY)"

agents-reset: ## Delete both agents' conversations and the host agent's notes; the WhatsApp link, the bookings and the audit logs stay
	$(COMPOSE) exec -T openclaw python3 - < openclaw/agents-reset.py

status: ## Show the services and whether WhatsApp is connected
	$(COMPOSE) ps
	$(COMPOSE) exec openclaw node openclaw.mjs channels status

logs: ## Follow every service's log
	$(COMPOSE) logs -f --tail 100

audit: ## Follow the Guardian's decisions: its envelope log, every signed request and answer (both Guardians)
	$(COMPOSE) run --rm --no-deps --entrypoint tail setup -n 20 -f /setup/audit/envelopes.jsonl

test: test-rules test-fake-pms test-live ## Run the tests that need no running stack (Docker only)

test-rules: ## The rules' case set and phone vectors, in OPA
	./guardian/test.sh

test-fake-pms: ## The fake PMS's Go tests
	docker run --rm -v "$(CURDIR):/src" -v acs-openclaw-demo-go-cache:/root/go -w /src/fake-pms \
		golang:1.27.3-alpine@sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd go test ./...

test-live: ## The talk's scenarios on the real rules against a running fake PMS
	./guardian/test-live.sh

# The image compose.yaml names for openclaw; the running container may hold an older ID of the same content.
OPENCLAW_IMAGE := acs-openclaw-demo-openclaw

test-history: ## Check the installed plugin's history in isolated test state; sends no WhatsApp messages
	docker run --rm -i --network none --entrypoint node $(OPENCLAW_IMAGE) --input-type=module < openclaw/test-history.mjs

test-send: ## Check the installed send handler and target refusal; sends no WhatsApp messages
	docker run --rm -i --network none --entrypoint node $(OPENCLAW_IMAGE) --input-type=module < openclaw/test-send.mjs

test-guardian: ## The talk's scenarios through the installed ACS adapter to the running Guardian and fake PMS (after make up)
	$(COMPOSE) exec -T openclaw node --input-type=module < openclaw/test-guardian.mjs

test-stack: ## The running OpenClaw: its settings, the adapter's hooks, both plugins in the live gateway (after make up)
	$(COMPOSE) exec -T openclaw sh -c 'node openclaw.mjs plugins inspect acs-guardian-client --runtime --json > /tmp/acs-runtime.json && python3 /opt/demo/check-config.py /home/node/.openclaw/openclaw.json /home/node/.openclaw/guardian.json /tmp/acs-runtime.json'
	@# The gateway's own start line names the plugins it loaded; a plugin that failed to load is missing from it.
	@line="$$($(COMPOSE) logs --no-log-prefix openclaw | grep 'http server listening' | tail -n 1)"; \
	for plugin in acs-guardian-client whatsapp-for-business; do \
		case "$$line" in *"$$plugin"*) ;; *) echo "The running OpenClaw did not load $$plugin: $$line" >&2; exit 1 ;; esac; \
	done

.PHONY: help up local-up start require-adapter down bookings-move agents-reset status whatsapp-link whatsapp-import logs audit test test-rules test-fake-pms test-live test-stack test-guardian test-history test-send
