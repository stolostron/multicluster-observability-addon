# Contribute

## Bootstrap Multiple Clusters Using the Log Storage Toolbox

In the [Log Storage
Toolbox](https://gitlab.cee.redhat.com/openshift-logging/log-storage-toolbox)
project on GitLab (behind VPN), we have a set of scripts that allow us to
provision OCP clusters in multiple cloud providers. The
[README.md](https://gitlab.cee.redhat.com/openshift-logging/log-storage-toolbox/-/blob/main/manifests/ocp/README.md)
under `log-storage-toolbox/manifests/ocp` contains detailed instructions on how
to use the script. **NOTE:** Be sure to follow the instructions in the `base`
section as they are prerequisites.

For development on the multicluster-observability-addon, most of the time it's
handy to provision two clusters, one hub and one spoke.

```shell
# Download openshift-install client
./scripts/ocp-download-release.sh 4.15.15
# Prepare bootstrap resources
./scripts/ocp-install.sh aws eu-central-1 4.15.15
# Launch cluster (takes 40min +-)
openshift-install-linux-4.15.15 create cluster --dir ./output/jmarcalaws24011261
./scripts/ocp-install.sh aws eu-central-1 4.15.15
openshift-install-linux-4.15.15 create cluster --dir ./output/jmarcalaws24011221
```

Both clusters are supported by `6` nodes `m6a.4xlarge` (3 masters, 3 workers).
It's indifferent which cluster you pick to be the hub and the spoke. When you
are done with developing, be sure to destroy them.

```shell
openshift-install-linux-4.15.15 destroy cluster --dir ./output/jmarcalaws24011261
openshift-install-linux-4.15.15 destroy cluster --dir ./output/jmarcalaws24011221
```

# Bootstrapping ACM and Linking Clusters

All steps are meant to be run on the hub cluster except when explicitly stated.

1. Use the OpenShift Installer to create and set up two OCP clusters.
2. Install the `Advanced Cluster Management for Kubernetes` operator.
3. Create a `MultiClusterHub` resource using the web console.
4. Import each spoke cluster to RHACM via the web console (top left, to the
   right of the RH OpenShift logo), using the commands option by running the
   commands on each spoke cluster.

## Development Cycle for Smoke Testing

Note: the addon has a dependency on cert-manager operator, which should be
installed on the hub cluster

When working on the addon, it's nice to be able to test things quickly; to do
this, you can:

```shell
export REGISTRY_BASE=quay.io/YOUR_QUAY_ID
oc create namepace open-cluster-management-observability
# Builds and pushes the addon images
make oci 
# Deploys the CRDs necessary, the addon using your built image
make addon-deploy 
```

Then every time you want to test a new version, you can just:

```shell
make oci
# Delete the mcoa pod which will make the Deployment pull the new image
oc -n open-cluster-management-observability delete pod -l app=multicluster-observability-addon-manager
```

### Enable specific Observability Capabilities

The addon supports enabling observability capabilities using the resource `AddOnDeploymentConfig`. For instance, to enable platform and user workloads logging/tracing/instrumentation create the following resource on the hub cluster:

```yaml
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: AddOnDeploymentConfig
metadata:
  name: multicluster-observability-addon
  namespace: open-cluster-management-observability
spec:
  customizedVariables:
  # Platform Observability
  - name: platformLogsCollection
    value: clusterlogforwarders.v1.observability.openshift.io
  # User Workloads Observability
  - name: userWorkloadLogsCollection
    value: clusterlogforwarders.v1.observability.openshift.io
  - name: userWorkloadTracesCollection
    value: opentelemetrycollectors.v1beta1.opentelemetry.io
  - name: userWorkloadInstrumentation
    value: instrumentations.v1alpha1.opentelemetry.io
``` 

Supported keys are:
- `platformLogsCollection`: Supports values `clusterlogforwarders.v1.observability.openshift.io`
- `userWorkloadLogsCollection`: Supports values `clusterlogforwarders.v1.observability.openshift.io`
- `userWorkloadTracesCollection`: Supports values `opentelemetrycollectors.v1beta1.opentelemetry.io`
- `userWorkloadTracesInstrumentation`: Supports values `instrumentations.v1alpha1.opentelemetry.io`

__Note__: Some keys can hold multiple values separated by semicolon to support multiple data collection capabilities in parallel, e.g:

```yaml
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: AddOnDeploymentConfig
metadata:
  name: multicluster-observability-addon
  namespace: open-cluster-management-observability
spec:
  customizedVariables:
  # User Workloads Observability with multiple collectors
  - name: userWorkloadLogsCollection
    value: clusterlogforwarders.v1.observability.openshift.io;opentelemetrycollectors.v1beta1.opentelemetry.io
```

The addon installation is managed by the addon-manager. This means that users
don't need to explicitly create resources to install the addon on spoke
clusters, the only requirements is that the clusters have to belong to a managed
cluster set. By default MCOA will be installed on all the cluster managed by the
hub. This behaviour can be changed by editing the `ClusterManagementAddOn`
resource in the `installStrategy` section.

The only resources the user has to create are:

1. Stanzas for `ClusterLogForwarder` and `OpenTelemetryCollector`;
2. Secrets for the configured `Outputs`/`Exporters`; Secrets can be created either on the namespace of a spoke cluster or in the same namespace as the stanza.
1. Once a `ManagedClusterAddon` (resource created by the addon-manager) is reconciled successfuly by MCOA we can
   look for the `ManifestWorks`

```shell
oc -n spoke-1 get manifestworks addon-multicluster-observability-addon-deploy-0
```

### Configuring Logs

Currently MCOA supports deploying a single instance of `ClusterLogForwarder`
templated with the stanza created in the hub cluster. The instance deployed in
the spoke cluster will have an exact copy of the `Spec` from the stanza.

This MCOA supports all outputs defined in [OpenShift Documentation](https://docs.openshift.com/container-platform/latest/observability/logging/log_collection_forwarding/configuring-log-forwarding.html)([API Ref](https://github.com/openshift/cluster-logging-operator/blob/master/api/logging/v1/output_types.go#L22-L43)). Furthermore, since MCOA will simply ship the specified secrets together with `ClusterLogForwarder` MCOA is also able to support all authentication methods supported by `ClusterLogForwarder`.

Note: the service account used by the `ClusterLogForwarder` deployed by MCOA is `openshift-logging/mcoa-logcollector`, this information is esential when using the AWS STS authentication.

### Traces Collection

Currently MCOA supports deploying a single instance of `OpenTelemetryCollector`
templated with the stanza created in the hub cluster. The instance deployed in
the spoke cluster will have an exact copy of the `Spec` from the stanza.

This MCOA supports all components defined in [OpenShift Documentation](https://docs.openshift.com/container-platform/latest/observability/otel/otel-configuration-of-otel-collector.html#otel-collector-components_otel-configuration-of-otel-collector). Furthermore, since MCOA will simply ship the specified secrets together with `OpenTelemetryCollector` MCOA is also able to support all authentication methods supported by `OpenTelemetryCollector`.

### Test files

Under the `hack` folder there are resources for helping manually testing the operator. For instance, there is a helm chart named `addon-install` for installing the necessary resources for the common use addon use-case. You can install that chart by running:

```shell
helm -n default upgrade --install addon-install hack/addon-install/
```

Note: Be sure to fill in the `values.yaml` file to configure the resources propperly otherwise you might endup with an installtion that doesn't actually work.

If you modify the chart you can the run the same command to apply the new version to the cluster. To clean up you can run the following:

```shell
helm -n default uninstall addon-install
```
