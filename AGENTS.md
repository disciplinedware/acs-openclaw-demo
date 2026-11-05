# Set up this demo for a person

Use this procedure when a person asks you to set up or run this demo. Do all the steps yourself. Ask the person only for the values that only they have. The [README](README.md) tells what the demo shows, and its "Terms" section defines the words used here.

1. Check the tools: run `docker version`, `docker compose version` and `make --version`. The README section "Run it" gives the tested versions.
2. Ask the person for these values, in one message:
   - The business phone: the WhatsApp number that OpenClaw replies on. A WhatsApp Business account is best.
   - The host phone: their own phone. It gets the approval cards, and they approve with 👍.
   - An Anthropic or an OpenAI API key.
   - The guest phone: the phone that plays the guest with reservation CC-1042, in scenarios 2 and 3.
   - Tell them that scenario 1 needs one more phone, the stranger phone. The host phone cannot be the guest phone or the stranger phone: its messages go to the host agent.
3. Run `cp .env.example .env`. Set `BUSINESS_PHONE`, `HOST_PHONE`, the API key and `GUEST_PHONE`. Write each phone in E.164 format: a plus and digits, with no spaces. For an OpenAI key, set `AGENT_MODEL` and `HOST_AGENT_MODEL` to `openai/<model>`. Do not change the other values: `make up` fills in the generated block.
4. Run `make up`. The first build takes some time. At the end, `make up` runs `make test-stack` and `make test-guardian`. If `make up` fails, read the error:
   - If the error names a value in `.env`, correct that value and run `make up` again.
   - For all other errors, show the error to the person and stop. Do not change the rules or the settings to make a check pass.
5. If today is not 2026-11-11, run `make bookings-move`. Then reservation CC-1042 arrives today. The door code opens for this guest two hours before check-in.
6. Run `make whatsapp-link` and show the QR code to the person. On the business phone, they open WhatsApp > Settings > Linked devices > Link a device and scan the QR code. This is the only step that you cannot do for them.
7. Run `make status`. The output must show `WhatsApp default: … connected`.
8. Tell the person to write to the business number from the guest phone or the stranger phone. The README section "What it shows" gives the messages of the three scenarios. `make audit` shows each decision of the Guardian.

Do not show the values of `.env` or `.secrets/` in the conversation. `make help` lists all commands. If the person has questions, they can write to the author on LinkedIn (README section "Who built this").
