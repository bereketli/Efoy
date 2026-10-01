{{/* Common labels. */}}
{{- define "efoy.labels" -}}
app.kubernetes.io/part-of: efoy
app.kubernetes.io/version: {{ .Values.image.tag | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end }}

{{/* Selector labels for one component: include "efoy.selector" (list $ "core-api"). */}}
{{- define "efoy.selector" -}}
{{- $root := index . 0 -}}
app.kubernetes.io/name: {{ index . 1 }}
app.kubernetes.io/instance: {{ $root.Release.Name }}
{{- end }}

{{/* Resource name for a component: <release>-<component>. */}}
{{- define "efoy.name" -}}
{{- printf "%s-%s" (index . 0).Release.Name (index . 1) | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{/* Image for a component. */}}
{{- define "efoy.image" -}}
{{- $root := index . 0 -}}
{{- printf "%s/efoy-%s:%s" $root.Values.image.registry (index . 1) $root.Values.image.tag -}}
{{- end }}

{{/* Env sources shared by the Go services and the migration job. */}}
{{- define "efoy.envFrom" -}}
- configMapRef:
    name: {{ .Release.Name }}-config
- secretRef:
    name: {{ .Values.existingSecret }}
{{- end }}
