# The TypeScript Guardian reads its key from the environment only.
ACS_HMAC_SECRET="$(cat /run/secrets/acs-hmac)"
export ACS_HMAC_SECRET
exec bun run guardian
