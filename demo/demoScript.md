# Demo: Multicluster Observability AddOn

Welcome to the demo of multicluster-observability-addon (MCOA)! MCOA is an addon for the Open Cluster Management ecosystem which RedHat has a distribution of known as RedHat Advanced Cluster Management (a.k.a ACM) which supports architectures where a fleet of clusters are controlled by a single hub cluster.

The goal of MCOA is to configure a fleet of spoke clusters to collect and forward observability signals to a configurable set of stores. MCOA achieves this by leveraging APIs and Operators already present in the single OCP cluster user-story.

In this demo, we will leverage MCOA to collect and send:
- Metrics to the hub cluster using Prometheus Agent;
- Logs to cloudwatch and to an instance of Loki running on the hub cluster using the `ClusterLogForwarder`;
- Traces to an instance of OTEL Collector running on the hub cluster using the `OpenTelemetryCollector`;

**Disclaimer**: for this demo we have already pre-provisioned both the hub and spoke clusters, we have already connected the two and we have already deployed some configuration for the stores. However, all the steps we just mentioned are also described in the README that exists on the demo fodler on the project repo.

## 1. Install multicluster-observability-addon

First, we are going to install the addon-manager on the hub cluster. The process of installing the manager on the hub cluster is quite simple. We just have to run:

RUN:
```bash
make addon-deploy
```

This command will:

- create RBAC resources necessary for the manager,
- the manager Deployment,
- and a resource called `ClusterManagementAddOn` which contains some defaults that will be used by the manager when installing the addon on a spoke cluster.

-- More detail beginning --
** SHOW `ClusterManagementAddOn` with `oc get ClusterManagementAddOn multicluster-observability-addon -o yaml` **

The `ClusterManagementAddOn` contains a list of resources that the cluster-admin can use to configure the addon deployment on each spoke cluster. In our case, we can see that our addon supports `addondeploymentconfigs, configmaps, secrets, clusterlogforwarders and opentelemetrycollectors`. And we set defaults for `addondeploymentconfigs, clusterlogforwarders, opentelemetrycollectors`.
-- More detail end --

Now that we have deployed the manager it's time to configure our signals collectors.

## 2. Manage observability signals across spoke clusters

### 2.1 Signal configuration

As previously mentioned MCOA uses APIs familiar to the single OCP cluster use case, to achieve it's goals.
This means that, for instance, if we want to configure log forwarding for our fleet then have to create a `ClusterLogForwarder` that will be used as a template to configure `ClusterLogForwarder` on the spoke clusters.

With in mind, for the demo we will:

- For metrics we will not need to create any resource as the metrics deployment nowadays re-uses the infrastructure already deployed by MCO;

- For logs we will deploy a `ClusterLogForwarder` that is configured to forward logs to all logs to CloudWatch and infrastructure logs to Loki;

- For traces we will deploy a `OpenTelemetryCollector` that is configured to forward logs to OTEL Collector;

For both logs and traces we also create a configmap that will store which authentication methods should be used against each store. For instance,

- For CloudWatch we will want to use static authentication

- For Loki we will want to use mTLS

MCOA will consume these configmaps and it will generate the necessary secrets for the different signals to interact with the signal stores.

For this demo we will deploy these configurations using a helm chart. We've pre-defined the values in `demo/addon-config/values.yaml`

If I run the deployment command with `--dry-run` we will be able to see the resources that are created 

RUN `helm upgrade --install addon-config demo/addon-config/ --dry-run`

@Joao go through logging resources

As previously mentioned our CLF is configured to forward logs to both CloudWatch and the Loki instance we have running on the hub. Notice that we leave with `PLACEHOLDER` all the fields that will be overwriten with cluster specific information once we install the addon on a spoke cluster. In this case the `url` of the Loki instance and the secret name that will be used for the communication.

We also create a configmap with the mapping between the signal store and the authentication method

@Israel go through tracing resources

TO BE DONE

Finally we can now install these resouces. Notice that these resources only need to be configured once, afterwards they will be the template that will be used by the different installations to generat the final manifests that will land on the spoke clusters.

RUN `helm upgrade --install addon-config demo/addon-config/`

### 2.2 Enable the addon for spoke clusters

Now that we have deployed the configuration for the addon its time to finally install the addon on the spoke `luster.
The process of installing an addon on a cluster is simple, we only need to create a resource called `ManagedClusterAddOn` in the namespace of the spoke cluster. This resource contains a list of resources that will be used to configure the addon deployment on the spoke cluster. 

For this demo we will install the addon using a helm chart. We've pre-defined the values in `demo/addon-install/values.yaml`

So now if I run the deployment command with `--dry-run` we will be able to see the resources that are created.

RUN `helm upgrade --install addon-install demo/addon-install/ --dry-run`

@Joao go through logging resources

On the Logging side we can se that `ManagedClusterAddOn` is configured to 

@Israel go through tracing resources

TO BE DONE


## 3. Validate with Grafana

https://grafana-route-grafana-operator.apps.jmarcalaws24021929.devcluster.openshift.com/d/eH_o-ZoSz/mcoa?orgId=1&from=now-5m&to=now