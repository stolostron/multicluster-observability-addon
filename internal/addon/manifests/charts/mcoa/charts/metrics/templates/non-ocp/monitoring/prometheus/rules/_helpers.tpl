{{/*
Common labels for prometheus rules.
*/}}
{{- define "prometheus-rules.labels" -}}
{{- include "metricshelm.labels" . }}
app.kubernetes.io/name: prometheus-rules
app.kubernetes.io/component: rules
{{- end -}}
