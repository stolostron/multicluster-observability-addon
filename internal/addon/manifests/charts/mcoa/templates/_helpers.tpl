
{{- define "mcoahelm.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}


{{- define "mcoahelm.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "mcoahelm.installCOO" -}}
{{- if and .Values.enabled .Values.analytics.incidentDetection.enabled -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}