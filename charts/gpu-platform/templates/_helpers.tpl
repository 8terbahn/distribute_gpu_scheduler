{{/*
Expand the name of the chart.
*/}}
{{- define "gpu-platform.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "gpu-platform.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "gpu-platform.labels" -}}
helm.sh/chart: {{ include "gpu-platform.name" . }}-{{ .Chart.Version }}
app.kubernetes.io/name: {{ include "gpu-platform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "gpu-platform.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gpu-platform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
