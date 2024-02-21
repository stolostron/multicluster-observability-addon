# Demo: Multicluster Observability AddOn
@Douglas: run the introduction 

Welcome to the demo of multicluster-observability-addon (aka MCOA). The MCOA is an addon for the Open Cluster Management project. Red Hat has its own distribution of this project, called Red Hat Advanced Cluster Management (aka ACM), which supports architectures where a fleet of clusters is controlled by a single hub cluster.

The goal of the MCOA is to configure a fleet of spoke clusters to collect and forward observability signals to a configurable set of stores. It achieves this by leveraging APIs and Operators already present in the OpenShift Platform clusters.

In this demo, we will leverage the MCOA to collect and send:

- Metrics to the hub cluster using Prometheus Agent.
- Logs to cloudwatch and to an instance of Loki running on the hub cluster using the `ClusterLogForwarder`.
- Traces to an instance of OTEL Collector running on the hub cluster using the `OpenTelemetryCollector`.

**Disclaimer**: for this demo we have already pre-provisioned both the hub and spoke clusters, connected them, and deployed some configuration for the stores. However, all the steps we just mentioned are also described in the README that exists on the demo folder of this project.

## 1. Install multicluster-observability-addon
@Douglas: explain the installation of the addon.

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

The `ClusterManagementAddOn` contains a list of resources that can be used to configure the addon deployment in each spoke cluster. The addon supports:  `addondeploymentconfigs`, `configmaps`, `secrets`, `clusterlogforwarders`, and `opentelemetrycollectors`..
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

If I run the deployment command with `--dry-run` we will be able to see the resources that will be created 

RUN `helm upgrade --install addon-config demo/addon-config/ --dry-run`

@Joao go through logging resources

As previously mentioned our CLF is configured to forward logs to both CloudWatch and the Loki instance we have running on the hub. Notice that we leave with `PLACEHOLDER` all the fields that will be overwriten with cluster specific information once we install the addon on a spoke cluster. In this case the `url` of the Loki instance and the secret name that will be used for the communication.

We also create a configmap with the mapping between the signal store and the authentication method

@Israel go through tracing resources

TO BE DONE

Finally we can now install these resouces. Notice that these resources only need to be configured once, afterwards they will be the template that will be used by the different installations to generat the final manifests that will land on the spoke clusters.

RUN `helm upgrade --install addon-config demo/addon-config/`

### 2.2 Enable the addon for spoke clusters

Now that we have deployed the configuration for the addon its time to finally install the addon on the spoke clusters.
The process of installing an addon on a cluster is simple, we only need to create a resource called `ManagedClusterAddOn` named `multicluster-observability-addon` in the namespace of the spoke cluster that we want to install the addon on. This resource contains a list of resources that will be used by the manager to configure the addon deployment on the spoke cluster.

For this demo we will install the addon using a helm chart. We've pre-defined the values in `demo/addon-install/values.yaml`

So now if I run the deployment command with `--dry-run` we will be able to see the resources that will be created.

RUN `helm upgrade --install addon-install demo/addon-install/ --dry-run`

@Joao go through the logging resources

For the logging configuration we will use:
- The `ClusterLogForwarder` created in the last step, it's not on the list because if you remember it was a default on the `ClusterManagementAddOn` resource.
- The authentication configmap `logging-auth` this is a configmap that has a mapping between the log stores on CLF and their authentication methods
- A configmap with the URL of the loki store, specific to the spoke cluster
- A configmap that will contain the CA, of the loki store, that will be injected in the mTLS secret used by CLF

@Israel go through tracing resources

TO BE DONE

Finally we can now install the addon. 

RUN `helm upgrade --install addon-install demo/addon-install/`

Once this finishes running the manager will reconcile the resources we just created and it will create a resource called `ManifestsWorks`, in the namespace of the cluster. This resource will contain a list of Kubernetes resources in YAML format that will be deployed to the spoke cluster by an agent running on the spoke cluster (the agent is installed on the spoke cluster when we import the spoke cluster to the hub cluster).

We can look at the `ManifestsWorks` created by running:

RUN `oc -n spoke-1 get manifestworks addon-multicluster-observability-addon-deploy-0 -o yaml`

Now we can see what is being installed on the spoke cluster.

- Metrics 

@Douglas: go through the resources being installed.

For the metrics collection we are installing:

- A `Deployment` with Prometheus in agent mode.
- A `ConfigMap` to instruct this Prometheus Agent to federate metrics from the Cluster Monitoring Operator's Prometheus, apply an allowlist, and remote write them to Observatorium in the Hub.
- A `Secret` with the TLS certificates needed to authenticate with Observatorium in the Hub.
- A `Secret` with the CA certificate to verify the identity of the Observatorium instance.

- Logs

- Logs

@Joao describe what's being installed

- Traces

@Israel describe what's being installed

Now let's jump into the spoke cluster and we see is the different signal collectors are all running correctly:

First, Prometheus Agent

RUN `oc -n open-cluster-management-addon-observability get pods -l app.kubernetes.io/component=metrics-agent`

Second, Vector

RUN `oc -n openshift-logging get pods -l `

Third, OTEL Collector

RUN `oc -n spoke-otel get pods -l `

## 3. Validate with Grafana

Great! Now let's jump into a grafana instance running on the hub cluster to see a dashboard where we can see all the 3 signals

RUN `oc -n grafana-operator get route grafana-route`

**Note: user: `root` password: `secret`**

- First we have the total amount of container memory being used on the `kube-system` namespace.
- Second we have the logs being produced by all the containers on the `kube-system` namespace, and the actuall logs.
- Third we have a set of traces from the `kube-system` namespace. 

## 4. Takeaways

- All the heavy lifting we just saw in this demo was done by the operators we already know & love today, cluster-logging-operator, opentelemetry-operator. 
- Configuration will be uniform across the fleet as the main configuration is only defined once and then will be templated to fit each spoke cluster.
- Configured signal stores can then be queried from a single grafana instance.
