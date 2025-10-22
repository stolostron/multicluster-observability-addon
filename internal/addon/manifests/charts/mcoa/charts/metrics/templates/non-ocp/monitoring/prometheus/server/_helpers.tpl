

{{/*
Common labels for prometheus.
*/}}
{{- define "prometheus.labels" -}}
{{- include "metricshelm.labels" . }}
app.kubernetes.io/name: prometheus
app.kubernetes.io/component: prometheus
prometheus: k8s
{{- end -}}

{{/*
Selector labels for prometheus pods.
*/}}
{{- define "prometheus.selectorLabels" -}}
app.kubernetes.io/name: prometheus
app.kubernetes.io/component: prometheus
app.kubernetes.io/part-of: multicluster-observability-addon
{{- end -}}
