# meter-proxy: the universal prepaid-metering sidecar for non-Go Hanzo products.
# Builds the standalone stdlib-only binary from the metering + metering/proxy
# leaf modules. CGO off -> a static binary on scratch (~zero attack surface).
#
# BUILD CONTEXT = the metering/ module root (so both the client module and the
# proxy submodule are in context, and the proxy's `replace ... => ../` resolves):
#
#   # from commerce/metering/ :
#   docker buildx build --platform linux/amd64 \
#     -f proxy/Dockerfile -t ghcr.io/hanzoai/meter-proxy:<tag> .
#
# In CI this is built by arcd's BuildKit Job (NO GitHub builders).
FROM golang:1.24-alpine AS build
WORKDIR /src/metering

# Context root IS the metering module. Copy the client module files at /src/metering
# and the proxy submodule at /src/metering/proxy, matching `replace => ../`.
COPY go.mod ./
COPY *.go ./
COPY proxy/ ./proxy/

WORKDIR /src/metering/proxy
ENV CGO_ENABLED=0 GOOS=linux
ARG TARGETARCH
RUN GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" \
    -o /out/meter-proxy ./cmd/meter-proxy

# Distroless-equivalent: scratch + the static binary. No shell, no libc.
FROM scratch
COPY --from=build /out/meter-proxy /meter-proxy
# The proxy listens on METER_PROXY_LISTEN (default :8080); the deploy template
# pins it to <product-port>+10000.
EXPOSE 8080
ENTRYPOINT ["/meter-proxy"]
