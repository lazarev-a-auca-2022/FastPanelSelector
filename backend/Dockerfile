FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO_ENABLED=0 works here specifically because modernc.org/sqlite is a
# pure-Go driver (no cgo) — the result is a static binary that runs on the
# distroless base below with no libc dependency.
#
# -p=1 caps build parallelism to one package at a time. modernc.org/libc
# (a large generated Go translation of C sqlite, pulled in by
# modernc.org/sqlite) needs a lot of RAM to compile; building it alongside
# other packages in parallel is a common OOM-kill cause on small VPS build
# hosts. This trades a slower build for one that actually completes on
# constrained memory — the runtime binary is unaffected either way.
RUN CGO_ENABLED=0 GOOS=linux go build -p=1 -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
