# The ACS TypeScript Guardian (reference-implementations/agt) at the adapter's
# commit, which carries PR #229: an ask names its approver. Bun 1.3 is the
# release the reference implementation's CI runs. The Guardian reads the
# specification's schemas by their path in the repository, so the layout stays.
FROM oven/bun:1.3.14-debian@sha256:9dba1a1b43ce28c9d7931bfc4eb00feb63b0114720a0277a8f939ae4dfc9db6f
WORKDIR /acs/reference-implementations/agt
COPY --from=acs specification/ /acs/specification/
COPY --from=acs reference-implementations/agt/ ./
RUN bun install --frozen-lockfile && mkdir /data && chown 65532:65532 /data
COPY start.sh /usr/local/bin/acs-guardian-ts
USER 65532:65532
ENTRYPOINT ["sh", "/usr/local/bin/acs-guardian-ts"]
