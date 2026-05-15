# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.25.8 as builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -mod=readonly -a -o multicluster-observability-addon main.go

# Distroless final image has no shell; use a minimal stage so COPY can materialize the symlink target.
FROM busybox:1.36 AS symlink-prep
WORKDIR /
COPY --from=builder /workspace/multicluster-observability-addon /multicluster-observability-addon
RUN mkdir -p /usr/local/bin && ln -s /multicluster-observability-addon /usr/local/bin/endpoint-monitoring-operator

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=symlink-prep /multicluster-observability-addon .
COPY --from=symlink-prep /usr/local/bin/endpoint-monitoring-operator /usr/local/bin/endpoint-monitoring-operator
USER 65532:65532

ENTRYPOINT ["/multicluster-observability-addon", "controller"]
