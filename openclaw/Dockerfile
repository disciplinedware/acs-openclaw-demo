# syntax=docker/dockerfile:1
# OpenClaw as the host runs it: OpenClaw, the WhatsApp channel plugin
# whatsapp-for-business and the ACS adapter, plus this demo's configuration.
# Build contexts (compose.yaml): `openclaw-core` is the pinned official
# release image; `whatsapp-plugin-src` is the fork that holds
# whatsapp-for-business; `acs`
# is the Agent Control Standard source that holds the OpenClaw adapter;
# `guardian` is this repository's guardian/ folder.

# whatsapp-for-business, packed by OpenClaw's own pack script. Node 24 with
# npm 11: npm 10 runs Baileys' prepare and fails. OpenClaw requires Node
# >=24.22.1 <25 || >=26.12.1.
FROM node:24.22.2-trixie-slim@sha256:54f84b4cf8432b21aaffd5d3ce8d41e12cea0d10d33c09d2f3c9879d49d6b446 AS whatsapp-plugin
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
WORKDIR /src
COPY --from=whatsapp-plugin-src . ./
RUN --mount=type=cache,id=openclaw-pnpm-store,target=/root/.local/share/pnpm/store \
    corepack enable pnpm \
    && pnpm install --frozen-lockfile \
    && OPENCLAW_PLUGIN_NPM_PACK_OUTPUT_DIR=/out bash scripts/plugin-npm-publish.sh --pack extensions/whatsapp \
    && mv /out/disciplinedware-whatsapp-for-business-*.tgz /out/whatsapp-for-business.tgz

# Official OpenClaw image, pinned by digest in compose.yaml.
FROM openclaw-core

USER root
RUN apt-get update && apt-get install -y --no-install-recommends python3-yaml rsync && rm -rf /var/lib/apt/lists/*
COPY session-entrypoint.sh /usr/local/bin/openclaw-session

# Plugins are installed into the image, not into OpenClaw's state volume, so a
# rebuild is the upgrade. OpenClaw's own parallel plugin gives the host agent
# web_search without a key (provider parallel-free). The flags and the `openclaw` link are what
# `openclaw plugins install` does for an npm plugin; the tarball bundles its
# runtime dependencies.
WORKDIR /opt/openclaw-plugins
COPY --from=whatsapp-plugin /out/whatsapp-for-business.tgz /tmp/whatsapp-for-business.tgz
# OpenClaw's parallel plugin at the core's release, pinned by the tarball's hash; it has no dependencies.
ADD --checksum=sha256:078a4bb61590d46a969367ae8930f4055cdab50e558e8234870f5164ddd989a9 \
	https://registry.npmjs.org/@openclaw/parallel-plugin/-/parallel-plugin-2026.10.9.tgz /tmp/parallel-plugin.tgz
RUN npm install --omit=dev --omit=peer --legacy-peer-deps --ignore-scripts --no-audit --no-fund \
		/tmp/whatsapp-for-business.tgz /tmp/parallel-plugin.tgz \
	&& rm /tmp/whatsapp-for-business.tgz /tmp/parallel-plugin.tgz \
	&& ln -s /app node_modules/openclaw

# The ACS adapter imports the TypeScript reference's host-adapter and the
# specification's JSON schemas by their relative paths in the ACS source, so
# all keep their places under /opt/acs.
COPY --from=acs adapters/openclaw/package.json adapters/openclaw/openclaw.plugin.json adapters/openclaw/*.ts /opt/acs/adapters/openclaw/
COPY --from=acs reference-implementations/agt/packages/host-adapter/package.json /opt/acs/reference-implementations/agt/packages/host-adapter/
COPY --from=acs reference-implementations/agt/packages/host-adapter/src /opt/acs/reference-implementations/agt/packages/host-adapter/src
COPY --from=acs specification/v0.1.0 /opt/acs/specification/v0.1.0
# The adapter's runtime dependencies at the exact versions its package.json
# names; its `openclaw/plugin-sdk/*` imports resolve to the running OpenClaw.
WORKDIR /opt/acs/adapters/openclaw
RUN npm install --omit=dev --omit=peer --ignore-scripts --no-audit --no-fund --no-package-lock \
	&& ln -s /app node_modules/openclaw

# Fixed releases of packages the base image and the plugins bundle.
COPY replace-bundled-packages.sh /tmp/replace-bundled-packages.sh
RUN sh /tmp/replace-bundled-packages.sh && rm /tmp/replace-bundled-packages.sh

# The demo's setup (setup.sh renders it from .env into the stack's volumes)
# and the WhatsApp link helper.
COPY setup.sh whatsapp-link.py check-config.py /opt/demo/
COPY config /opt/demo/openclaw/config
COPY --from=guardian guardian.yaml mapping.yaml pack.yaml /opt/demo/guardian/
COPY --from=guardian policy /opt/demo/guardian/policy

# Debian security fixes newer than the base image.
RUN apt-get update && apt-get upgrade -y && rm -rf /var/lib/apt/lists/*

WORKDIR /app
USER node
