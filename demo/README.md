# Demo: Multicluster Observability AddOn

The following steps demonstrate how to use the `multicluster-observability-addon` to collect and forward observability signals (metrics, logs and traces) across a Red Hat Advanced Cluster Management (RHACM) managed fleet of OpenShift (OCP) clusters. Currently the `multicluster-observability-addon` is limited to only collection and forwarding of signals. Thus step 1 & 2 are dedicated to configure both RHACM and `LokiStack` as the storage solution on the RHACM hub cluster.

All steps are meant to be run on the hub cluster except when explicitely said.

## 1. Prerequisites: RHACM and OCP cluster fleet

For this demo you will need at least two OCP clusters (hosted on AWS) with one of them (the hub) having at least machines of flavor `m6a.4xlarge` in order to have enough resources for `LokiStack`. You will also need an S3 Bucket in the same region as the hub cluster.
 
1. Use the OpenShift Installer to create and setup two OCP cluster on AWS.
1. Install the `Advanced Cluster Management for Kubernetes` operator.
1. Create a `MultiClusterHub` resource.
1. Import each spoke cluster `RHACM` via the web console, using the commands option by running the commands on each spoke cluster.

## 2. Configure stores on the hub cluster

The following steps use Helm to install a set of RHACM `ConfigurationPolicies` that facilitate the configuration of the different stores that will be used in this demo.

_Hint:_ `cert-manager` operator is also installed in this step on the hub cluster. It will be used to demonstrate the ability to delegate PKI management for all tenants to a third-party tool.

1. Set the values in `demo/mcoa-demo/values.yaml`
1. Deploy the chart `helm upgrade --install mcoa-demos demo/mcoa-demo/`. This Helm chart will bootstrap configuration on the hub cluster to enabled it to receive signals from the spoke clustes.
1. Run `oc label --overwrite managedcluster/local-cluster cluster.open-cluster-management.io/clusterset=hub-mcoa-clusters` to label the `local-cluster` a.k.a. hub so that the policy applies to it.

## 3. Install multicluster-observability-addon

1. Deploy the addon controller by running `make addon-deploy`.

## 4. Manage observability signals across spoke clusters

The following chart will deploy the `ManagedClusterAddOn` resource that installs the AddOn on each spoke cluster.

1. Set the values in `demo/addon-install/values.yaml`.
1. Deploy it with `helm upgrade --install addon-install demo/addon-install/`. 

## 5. Validate with Grafana
