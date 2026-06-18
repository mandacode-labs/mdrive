{{- define "mdrive.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "mdrive.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "mdrive.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "mdrive.labels" -}}
helm.sh/chart: {{ include "mdrive.chart" . }}
{{ include "mdrive.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "mdrive.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mdrive.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "mdrive.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "mdrive.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "mdrive.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion | toString -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end }}

{{- define "mdrive.configSecretName" -}}
{{- default (include "mdrive.fullname" .) .Values.secrets.existingSecret -}}
{{- end }}
