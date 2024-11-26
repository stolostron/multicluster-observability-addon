
{{- define "metricshelm.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}


{{- define "metricshelm.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "metricshelm.labels" }}
app: {{ template "metricshelm.name" . }}
chart: {{ template "metricshelm.chart" . }}
release: {{ .Release.Name }}
app.kubernetes.io/part-of: multicluster-observability-addon
app.kubernetes.io/version: {{ .Chart.Version }}
app.kubernetes.io/managed-by: multicluster-observability-addon-manager
{{- end }}
