{{- define "logginghelm.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}


{{- define "logginghelm.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "logginghelm.installCLO" -}}
{{- if and .Values.enabled (or .Values.managed.collection.enabled .Values.unmanaged.collection.enabled) -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{- define "logginghelm.installLO" -}}
{{- if and .Values.enabled .Values.managed.storage.enabled -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}
