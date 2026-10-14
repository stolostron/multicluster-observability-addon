package workload

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/prometheus/promql/promqltest"
)

func TestPodMappingWithRealPromQLEvaluation(t *testing.T) {
	// Include system namespaces, duplicate KSM scrapes, standalone pods, and
	// non-workload owners. Unknown identity must stay absent, not be invented.
	expr := strings.Join(strings.Fields(podWorkloadRelabelExpr(`namespace!=""`)), " ")
	promqltest.RunTest(t, fmt.Sprintf(`
load 1m
  kube_pod_info{namespace="openshift-test",pod="bare",instance="a"} 1
  kube_pod_info{namespace="openshift-test",pod="bare",instance="b"} 1
  kube_pod_info{namespace="test",pod="static"} 1
  kube_pod_info{namespace="test",pod="catalog"} 1
  kube_pod_info{namespace="test",pod="installer"} 1
  kube_pod_owner{namespace="test",pod="static",owner_kind="Node",owner_name="node1"} 1
  kube_pod_owner{namespace="test",pod="catalog",owner_kind="CatalogSource",owner_name="catalog1"} 1
  kube_pod_owner{namespace="test",pod="installer",owner_kind="ConfigMap",owner_name="config1"} 1
  kube_pod_info{namespace="test",pod="deployment-pod"} 1
  kube_pod_owner{namespace="test",pod="deployment-pod",owner_kind="ReplicaSet",owner_name="app-rs"} 1
  kube_replicaset_owner{namespace="test",replicaset="app-rs",owner_kind="Deployment",owner_name="app"} 1
  kube_pod_owner{namespace="test",pod="rs-pod",owner_kind="ReplicaSet",owner_name="bare-rs"} 1
  kube_pod_owner{namespace="test",pod="cron-pod",owner_kind="Job",owner_name="cron-job"} 1
  kube_job_owner{namespace="test",job_name="cron-job",owner_kind="CronJob",owner_name="cron"} 1
  kube_pod_owner{namespace="test",pod="job-pod",owner_kind="Job",owner_name="job"} 1
  kube_pod_owner{namespace="test",pod="ds-pod",owner_kind="DaemonSet",owner_name="ds",instance="a"} 1
  kube_pod_owner{namespace="test",pod="ds-pod",owner_kind="DaemonSet",owner_name="ds",instance="b"} 1
  kube_pod_owner{namespace="test",pod="sts-pod",owner_kind="StatefulSet",owner_name="sts"} 1
  kube_pod_owner{namespace="test",pod="rc-pod",owner_kind="ReplicationController",owner_name="rc"} 1

eval instant at 0m %s
  {namespace="openshift-test",pod="bare"} 1
  {namespace="test",pod="static"} 1
  {namespace="test",pod="catalog"} 1
  {namespace="test",pod="installer"} 1
  {namespace="test",pod="deployment-pod",workload="app",workload_type="Deployment"} 1
  {namespace="test",pod="rs-pod",workload="bare-rs",workload_type="ReplicaSet"} 1
  {namespace="test",pod="cron-pod",workload="cron",workload_type="CronJob"} 1
  {namespace="test",pod="job-pod",workload="job",workload_type="Job"} 1
  {namespace="test",pod="ds-pod",workload="ds",workload_type="DaemonSet"} 1
  {namespace="test",pod="sts-pod",workload="sts",workload_type="StatefulSet"} 1
  {namespace="test",pod="rc-pod",workload="rc",workload_type="ReplicationController"} 1
`, expr), promqltest.NewTestEngine(t, false, 5*time.Minute, 100000))
}
