# Multi Cluster Observability AddOn

## Description

The multicluster-observability-addon is a pluggable addon working on OCM
rebased on the extensibility provided by
[addon-framework](https://github.com/open-cluster-management-io/addon-framework)
which automates the collection and forwarding of observability signals to a
central stores.

This is acheived through the installation on the spoke clusters of dedicated operators for each observability signal: 

- For Metrics the addon will deploy an instance of Prometheus running in agent mode, that will forward metrics to the hub.

- For Logs the operator installed will be [cluster-logging-operator](https://github.com/openshift/cluster-logging-operator). And the AddOn will configure a [ClusterLogForwarder](https://github.com/openshift/cluster-logging-operator) resource to forward logs to AWS CloudWatch.

The logging-ocm-addon consists of one component:

- **Addon-Manager**: Not only manages the installation of the AddOn on spoke clusters. But also builds the manifests that will be deployed to the spoke clusters.

## Demo

TBD

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

## References

- Addon-Framework: [https://github.com/open-cluster-management-io/addon-framework](https://github.com/open-cluster-management-io/addon-framework)