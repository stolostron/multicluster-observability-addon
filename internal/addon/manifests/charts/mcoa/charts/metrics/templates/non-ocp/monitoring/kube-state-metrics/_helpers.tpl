{{/*
kube-state-metrics fullname.
*/}}
{{- define "kube-state-metrics.fullname" -}}
kube-state-metrics
{{- end -}}

{{/*
kube-state-metrics service account name.
*/}}
{{- define "kube-state-metrics.serviceAccountName" -}}
kube-state-metrics
{{- end -}}

{{/*
Common labels for kube-state-metrics.
*/}}
{{- define "kube-state-metrics.labels" -}}
{{- include "metricshelm.labels" . }}
app.kubernetes.io/name: kube-state-metrics
app.kubernetes.io/component: exporter
{{- end -}}

{{/*
Selector labels for kube-state-metrics. These are used for pod template labels as well.
*/}}
{{- define "kube-state-metrics.selectorLabels" -}}
app.kubernetes.io/name: kube-state-metrics
app.kubernetes.io/component: exporter
app.kubernetes.io/part-of: multicluster-observability-addon
{{- end -}}

{{/*
Map of apiGroups to resources for kube-state-metrics.
*/}}
{{- define "kube-state-metrics.collectors" -}}
{
  "": ["configmaps", "nodes", "pods", "services", "resourcequotas", "replicationcontrollers", "limitranges", "persistentvolumeclaims", "persistentvolumes", "namespaces"],
  "apps": ["statefulsets", "daemonsets", "deployments", "replicasets"],
  "batch": ["cronjobs", "jobs"],
  "autoscaling": ["horizontalpodautoscalers"],
  "policy": ["poddisruptionbudgets"],
  "certificates.k8s.io": ["certificatesigningrequests"],
  "storage.k8s.io": ["storageclasses", "volumeattachments"],
  "admissionregistration.k8s.io": ["mutatingwebhookconfigurations", "validatingwebhookconfigurations"],
  "networking.k8s.io": ["networkpolicies", "ingresses"],
  "coordination.k8s.io": ["leases"],
  "discovery.k8s.io": ["endpointslices"]
}
{{- end -}}
