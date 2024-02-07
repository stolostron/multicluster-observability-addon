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
./scripts/ocp-download-release.sh 4.14.7
# Prepare bootstrap resources
./scripts/ocp-install.sh aws eu-central-1 4.14.7
# Launch cluster (takes 40min +-)
openshift-install-linux-4.14.7 create cluster --dir ./output/jmarcalaws24011261
./scripts/ocp-install.sh aws eu-central-1 4.14.7
openshift-install-linux-4.14.7 create cluster --dir ./output/jmarcalaws24011221
```

Both clusters are supported by `6` nodes `m6a.4xlarge` (3 masters, 3 workers).
It's indifferent which cluster you pick to be the hub and the spoke. When you
are done with developing, be sure to destroy them.

```shell
openshift-install-linux-4.14.7 destroy cluster --dir ./output/jmarcalaws24011261
openshift-install-linux-4.14.7 destroy cluster --dir ./output/jmarcalaws24011221
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
# Builds and pushes the addon images
make oci 
# Deploys the CRDs necessary, the addon using your built image
make addon-deploy 
```

Then every time you want to test a new version, you can just:

```shell
make oci
# Delete the mcoa pod which will make the Deployment pull the new image
oc -n open-cluster-management delete pod -l app=multicluster-observability-addon-manager
```

### Disabeling specific signals 

The addon supports disabling signals using the resource `AddOnDeploymentConfig`. For instance, to disable the logging signal create the following resource on the hub cluster:

```yaml
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: AddOnDeploymentConfig
metadata:
  name: multicluster-observability-addon
  namespace: open-cluster-management
spec:
  customizedVariables:
    - name: loggingDisabled
      value: "true"
``` 

Supported keys are `metricsDisabled`, `loggingDisabled` and `tracingDisabled`

## Install the addon on a Spoke Cluster

To actually install the addon on a spoke cluster, you need to:

1. Have the addon manager running on the hub cluster.
2. Create the necessary Kubernetes resources in the namespace of the spoke
    cluster that will be used by the addon to generate the `ManifestWorks`, e.g.,
    `secrets`, `configmaps`.
3. Create the `ManagedClusterAddon` resource in the namespace of the spoke
    cluster.

```yaml
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ManagedClusterAddOn
metadata:
  name: multicluster-observability-addon
  namespace: spoke-1
spec:
  installNamespace: open-cluster-management-agent-addon
  configs:
  # This ConfigMap will contain the CloudWatch url where we want to forward our
  # logs to
  - resource: configmaps
    name: spoke-1
    namespace: spoke-1
  # This Secret will contain the mTLS contents that will be used to communicate
  # with CloudWatch
  - resource: secrets
    name: spoke-1
    namespace: spoke-1
```

4. Once a `ManagedClusterAddon` is reconciled successfuly by the addon we can
   look for the `ManifestWorks`

```shell
oc get manifestworks -n spoke-1
```
