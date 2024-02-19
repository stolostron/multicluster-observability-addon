# Demo: Multicluster Observability AddOn

Welcome to the demo of the multicluster-observability-addon. MCOA is an addon for the open-cluster-management ecosystem a.k.a by RedHat Advanced Cluster Management which supports architectures where a fleet of clusters are controlled by a single hub cluster.

The goal of the MCOA is to configure the spoke clusters to collect and forward observability signals to the hub cluster. MCAO achieves this by leveraging APIs and Operators already familiar to the single OCP cluster user-story.

In this demo, we will have a central hub cluster and a spoke cluster (`spoke-1`). We will install MCOA on the hub cluster and we will configure it so metrics, logs and traces from the spoke cluster land on the hub cluster.

## 1. Install multicluster-observability-addon

First, we are going to install the addon-manager on the hub cluster. The process of installing the manager on the hub cluster is quite simple.

RUN `make addon-deploy` 

This command will:

- create RBAC resources necessary for the manager, 
- the manager Deployment,
- and a resource called `ClusterManagementAddOn` which contains some defaults that will be used by the manager when installing the addon on a spoke cluster.

** SHOW `ClusterManagementAddOn` with `oc get ClusterManagementAddOn multicluster-observability-addon -o yaml` ** 

More detail:

The `ClusterManagementAddOn` contains a list of resources that the cluster-admin can use to configure the addon deployment on each spoke cluster. In our case, we can see that our addon supports `addondeploymentconfigs, configmaps, secrets, clusterlogforwarders and opentelemetrycollectors`. And we set defaults for `addondeploymentconfigs, clusterlogforwarders, opentelemetrycollectors`. 

## 2. Manage observability signals across spoke clusters

### 2.1 Signal configuration

After deploying the manager it's time to configure our signals collectors. As previously mentioned MCOA strives to re-use familiar APIs in the single OCP cluster use case, to achieve it's goals. 
This means that, for instance, if we want to configure log forwarding for our fleet then have to create a `ClusterLogForwarder` that will be used as a template to configure `ClusterLogForwarder` on the spoke clusters.

** SHOW `ClusterLogForwarder` **

As we can see our CLF is configured to forward logs to both CloudWatch and the Loki instance we have running on the hub. We leave with `PLACEHOLDER` all the fields that will be overwriten with cluster specific information once we install the addon on a spoke cluster.

Similarly we follow the same patter for traces.




For this demo we will be doing that through the use of a helm chart, I've pre-defined the values in `demo/addon-config/values.yaml`

RUN `helm upgrade --install addon-config demo/addon-config/`

### 2.2 Enable the addon for spoke clusters

The following chart will deploy the `ManagedClusterAddOn` resource that enables the AddOn on each spoke cluster.

1. Set the values in `demo/addon-install/values.yaml`.
1. Deploy it with `helm upgrade --install addon-install demo/addon-install/`. 

## 3. Validate with Grafana

https://grafana-route-grafana-operator.apps.jmarcalaws24021929.devcluster.openshift.com/d/eH_o-ZoSz/mcoa?orgId=1&from=now-5m&to=now