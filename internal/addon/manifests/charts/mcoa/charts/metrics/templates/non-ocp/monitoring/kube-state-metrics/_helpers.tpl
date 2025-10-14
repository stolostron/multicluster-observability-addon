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
