# Copies in a binary that was cross-compiled outside the container (see
# build.sh) rather than compiling Go inside the build. This exists because
# compiling modernc.org/sqlite's dependency modernc.org/libc (a large
# generated Go translation of C sqlite) needs more RAM than some small VPS
# build hosts have — it gets OOM-killed there even with restricted build
# parallelism. If you're building on a host with plenty of RAM (CI, a dev
# machine) and would rather build from source in-container, use
# Dockerfile.source instead: `docker-compose build --file Dockerfile.source`.
#
# Regenerate server-linux-amd64 after any Go code change: ./build.sh
#
# The chmod happens in this intermediate stage, not the final distroless
# image, because distroless has no shell to run it in — and it can't be
# skipped by just committing the binary with the right mode, because git on
# a Windows checkout with core.fileMode=false ignores the executable bit
# entirely, so the bit committed to the repo can't be relied on.
FROM alpine:3.20 AS prep
COPY server-linux-amd64 /server
RUN chmod 0755 /server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=prep /server /server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
