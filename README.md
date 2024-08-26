# Multi Cluster Observability AddOn

## Description

The multicluster-observability-addon is a pluggable addon working on OCM
based on the extensibility provided by
[addon-framework](https://github.com/open-cluster-management-io/addon-framework)
which automates the collection and forwarding of observability signals to central stores.

This is acheived through the installation on the spoke clusters of dedicated operators for each observability signal:

- For Metrics, currently the multicluster-observability-operator should be used.

- For Logs the operator installed will be [cluster-logging-operator](https://docs.openshift.com/container-platform/latest/logging/cluster-logging.html). The addon will also configure an instance of [ClusterLogForwarder](https://docs.openshift.com/container-platform/latest/logging/log_collection_forwarding/configuring-log-forwarding.html) to forward logs to a configured store.

- For Traces the operator installed will be [Red Hat build of OpenTelemetry](https://docs.openshift.com/container-platform/latest/otel/otel_rn/otel-rn-3.1.html). The addon will also configure an instance of [OpenTelemetryCollector](https://docs.openshift.com/container-platform/latest/otel/otel-configuration-of-otel-collector.html) to forward traces to a configued store.

The logging-ocm-addon consists of one component:

- **Addon-Manager**: Not only manages the installation of the AddOn on spoke clusters. But also builds the manifests that will be deployed to the spoke clusters.

## Getting started

### Prerequisite

- OCM registration (>= 0.5.0)

### Steps

#### Installing via Kustomize

1. Install the AddOn using Kustomize

    ```shell
    kubectl apply -k deploy/
    ```

1. The addon should now be installed in you hub cluster

    ```shell
    kubectl get ClusterManagementAddOn multicluster-observability-addon
    ```

1. The addon will install automatically in spoke clusters once the resources referenced in `ClusterManagementAddOn` are created.

#### Installing via MCO

In 2.12, multicluster-observability-operator has the ability to install MCOA using the [capabilities field](https://github.com/stolostron/multicluster-observability-operator/blob/5d1fc789df365b20951b5fe1c378b5eebb306390/operators/multiclusterobservability/api/v1beta2/multiclusterobservability_types.go#L187-L212).

1. Create a `MultiClusterObservability` resource and configure `capabilities`

    ```yaml
    apiVersion: observability.open-cluster-management.io/v1beta2
    kind: MultiClusterObservability
    metadata:
      name: observability
    spec:
      capabilities:
        platform:
          logs:
            collection:
              enabled: true
        userWorkloads:
          logs:
            collection:
              clusterLogForwarder:
                enabled: true
          traces:
            collection:
              instrumentation:
                enabled: true
              openTelemetryCollector:
                enabled: false
      observabilityAddonSpec: {}
      storageConfig:
        metricObjectStorage:
          name: thanos-object-storage
          key: thanos.yaml
    ```

    Note: Deploy a custom image by adding the annotation: `mco-multicluster_observability_addon-image: quay.io/YOUR_ORG_HERE/multicluster-observability-addon:YOUR_TAG_HERE`

1. The addon should now be installed in you hub cluster

    ```shell
    kubectl get ClusterManagementAddOn multicluster-observability-addon
    ```

1. The addon will install automatically in spoke clusters once the resources referenced in `ClusterManagementAddOn` are created.

## References

- Open-Cluster-Management: [https://github.com/open-cluster-management-io/ocm](https://github.com/open-cluster-management-io/ocm)
- Addon-Framework: [https://github.com/open-cluster-management-io/addon-framework](https://github.com/open-cluster-management-io/addon-framework)
- Multicluster-Observability-Operator: [https://github.com/stolostron/multicluster-observability-operator](https://github.com/stolostron/multicluster-observability-operator)
- Cluster-Logging-Operator: [https://github.com/openshift/cluster-logging-operator](https://github.com/openshift/cluster-logging-operator)
- OpenTelemetry-Operator: [https://github.com/open-telemetry/opentelemetry-operator](https://github.com/open-telemetry/opentelemetry-operator)
