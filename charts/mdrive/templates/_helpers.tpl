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

{{/*
GC CronJob container definition.
Params:
  .root   - chart root context
  .name   - container/job name (e.g. tombstones)
  .args   - extra CLI args after "gc" (list)
  .env    - extra env vars (optional)
*/}}
{{- define "mdrive.gc.container" -}}
- name: gc
  securityContext:
    {{- toYaml .root.Values.securityContext | nindent 4 }}
  image: {{ include "mdrive.image" .root }}
  imagePullPolicy: {{ .root.Values.image.pullPolicy }}
  command:
    - /app/mdrive
  args:
    - gc
    {{- range .args }}
    - {{ . | quote }}
    {{- end }}
    - --config
    - /app/config/config.yaml
  env:
    - name: DATABASE_PASSWORD
      valueFrom:
        secretKeyRef:
          name: {{ .root.Values.secrets.database.existingSecret | default (include "mdrive.fullname" .root) }}
          key: {{ .root.Values.secrets.database.existingSecretKey | default "database-password" }}
    {{- if or .root.Values.secrets.storage.access_key .root.Values.secrets.storage.existingSecret }}
    - name: STORAGE_ACCESS_KEY
      valueFrom:
        secretKeyRef:
          name: {{ .root.Values.secrets.storage.existingSecret | default (include "mdrive.fullname" .root) }}
          key: {{ .root.Values.secrets.storage.existingSecretKeyAccess | default "storage-access-key" }}
    - name: STORAGE_SECRET_KEY
      valueFrom:
        secretKeyRef:
          name: {{ .root.Values.secrets.storage.existingSecret | default (include "mdrive.fullname" .root) }}
          key: {{ .root.Values.secrets.storage.existingSecretKeySecret | default "storage-secret-key" }}
    {{- end }}
    {{- if or .root.Values.secrets.crypto.master_key .root.Values.secrets.crypto.existingSecret }}
    - name: CRYPTO_MASTER_KEY
      valueFrom:
        secretKeyRef:
          name: {{ .root.Values.secrets.crypto.existingSecret | default (include "mdrive.fullname" .root) }}
          key: {{ .root.Values.secrets.crypto.existingSecretKey | default "crypto-master-key" }}
    {{- end }}
    {{- with .env }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
    {{- with .root.Values.extraEnv }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
  volumeMounts:
    - name: config
      mountPath: /app/config
      readOnly: true
    - name: tmp
      mountPath: /tmp
  resources:
    {{- toYaml .resources | nindent 4 }}
{{- end }}

{{/*
GC CronJob spec wrapper.
Params:
  .root     - chart root context
  .name     - job suffix (e.g. tombstones)
  .schedule - cron expression
  .config   - job-specific values block
  .args     - extra CLI args after "gc" (list)
  .env      - extra env vars (optional)
*/}}
{{- define "mdrive.gc.cronjob" -}}
{{- if .config.enabled -}}
apiVersion: batch/v1
kind: CronJob
metadata:
  name: {{ include "mdrive.fullname" .root }}-gc-{{ .name }}
  labels:
    {{- include "mdrive.labels" .root | nindent 4 }}
  {{- with .config.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  schedule: {{ .schedule | quote }}
  concurrencyPolicy: {{ .config.concurrencyPolicy | default "Forbid" }}
  successfulJobsHistoryLimit: {{ .config.successfulJobsHistoryLimit | default 3 }}
  failedJobsHistoryLimit: {{ .config.failedJobsHistoryLimit | default 1 }}
  jobTemplate:
    spec:
      ttlSecondsAfterFinished: {{ .config.ttlSecondsAfterFinished | default 300 }}
      backoffLimit: {{ .config.backoffLimit | default 3 }}
      activeDeadlineSeconds: {{ .config.activeDeadlineSeconds | default 600 }}
      template:
        metadata:
          labels:
            {{- include "mdrive.selectorLabels" .root | nindent 12 }}
        spec:
          restartPolicy: Never
          {{- with .root.Values.imagePullSecrets }}
          imagePullSecrets:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          serviceAccountName: {{ include "mdrive.serviceAccountName" .root }}
          automountServiceAccountToken: {{ .root.Values.serviceAccount.automount }}
          securityContext:
            {{- toYaml .root.Values.podSecurityContext | nindent 12 }}
          containers:
            {{- include "mdrive.gc.container" (dict "root" .root "name" .name "args" .args "env" .env "resources" .config.resources) | nindent 12 }}
          volumes:
            - name: config
              configMap:
                name: {{ include "mdrive.fullname" .root }}
            - name: tmp
              emptyDir: {}
{{- end }}
{{- end }}
