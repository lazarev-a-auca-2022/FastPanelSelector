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
FROM gcr.io/distroless/static-debian12:nonroot
COPY server-linux-amd64 /server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
