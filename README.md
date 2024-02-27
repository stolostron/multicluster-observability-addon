# Multi Cluster Observability AddOn

## Description

The multicluster-observability-addon is a pluggable addon working on OCM
based on the extensibility provided by
[addon-framework](https://github.com/open-cluster-management-io/addon-framework)
which automates the collection and forwarding of observability signals to central stores.

This is acheived through the installation on the spoke clusters of dedicated operators for each observability signal: 

- For Metrics the addon will deploy an instance of Prometheus running in agent mode, that will forward metrics to the hub.

- For Logs the operator installed will be [cluster-logging-operator](https://docs.openshift.com/container-platform/latest/logging/cluster-logging.html). The addon will also configure an instance of [ClusterLogForwarder](https://docs.openshift.com/container-platform/latest/logging/log_collection_forwarding/configuring-log-forwarding.html) to forward logs to a configured store.

- For Traces the operator installed will be [Red Hat build of OpenTelemetry](https://docs.openshift.com/container-platform/latest/otel/otel_rn/otel-rn-3.1.html). The addon will also configure an instance of [OpenTelemetryCollector](https://docs.openshift.com/container-platform/latest/otel/otel-configuration-of-otel-collector.html) to forward traces to a configued store.

The logging-ocm-addon consists of one component:

- **Addon-Manager**: Not only manages the installation of the AddOn on spoke clusters. But also builds the manifests that will be deployed to the spoke clusters.

## Getting started

### Prerequisite

- OCM registration (>= 0.5.0)
- cert-manager operator
- multicluster-observability-operator (for metrics)

### Steps

#### Installing via Kustomize

1. Install the AddOn using Kustomize

```shell
$ kubectl apply -k deploy/
```

2. The addon should now be installed in you hub cluster 
```shell
$ kubectl get ClusterManagementAddOn multicluster-observability-addon
```

3. The addon can now be installed it managed clusters by creating `ManagedClusterAddOn` resources in their respective namespaces

## Demo

Steps to deploy a demo of the addon can be found at [demo/README.md](https://github.com/rhobs/multicluster-observability-addon/tree/main/demo#readme)

## References

- Addon-Framework: [https://github.com/open-cluster-management-io/addon-framework](https://github.com/open-cluster-management-io/addon-framework)