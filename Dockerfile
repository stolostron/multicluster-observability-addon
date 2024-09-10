FROM scratch

ARG TARGETARCH

WORKDIR /

# Copy binary built on the host
COPY bin/multicluster-observability-addon_${TARGETARCH} /multicluster-observability-addon

USER 65532:65532

ENTRYPOINT ["/multicluster-observability-addon"]
