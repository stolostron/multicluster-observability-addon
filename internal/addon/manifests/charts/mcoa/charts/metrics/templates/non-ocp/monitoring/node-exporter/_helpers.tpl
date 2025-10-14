{{/*
node-exporter fullname.
*/}}
{{- define "node-exporter.fullname" -}}
node-exporter
{{- end -}}

{{/*
node-exporter service account name.
*/}}
{{- define "node-exporter.serviceAccountName" -}}
node-exporter
{{- end -}}

{{/*
Common labels for node-exporter.
*/}}
{{- define "node-exporter.labels" -}}
{{- include "metricshelm.labels" . }}
app.kubernetes.io/name: node-exporter
app.kubernetes.io/component: exporter
{{- end -}}

{{/*
Selector labels for node-exporter. These are used for pod template labels as well.
*/}}
{{- define "node-exporter.selectorLabels" -}}
app.kubernetes.io/name: node-exporter
app.kubernetes.io/component: exporter
app.kubernetes.io/part-of: multicluster-observability-addon
{{- end -}}
