{{- define "deps.labels" -}}
app.kubernetes.io/part-of: efoy
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end }}

{{- define "deps.selector" -}}
app.kubernetes.io/name: {{ index . 1 }}
app.kubernetes.io/instance: {{ (index . 0).Release.Name }}
{{- end }}

{{/* A ClusterIP service and a single-replica StatefulSet with one volume.
     Usage: include "deps.store" (dict "root" $ "name" "redis" "port" 6379 "cfg" .Values.redis "pod" $podSpecYaml "mountPath" "/data") */}}
{{- define "deps.store" -}}
{{- $root := .root -}}
{{- $fullname := printf "%s-%s" $root.Release.Name .name -}}
apiVersion: v1
kind: Service
metadata:
  name: {{ $fullname }}
  labels:
    {{- include "deps.labels" $root | nindent 4 }}
spec:
  selector:
    {{- include "deps.selector" (list $root .name) | nindent 4 }}
  ports:
    {{- range $portName, $port := .ports }}
    - name: {{ $portName }}
      port: {{ $port }}
      targetPort: {{ $port }}
    {{- end }}
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ $fullname }}
  labels:
    {{- include "deps.labels" $root | nindent 4 }}
    {{- include "deps.selector" (list $root .name) | nindent 4 }}
spec:
  serviceName: {{ $fullname }}
  replicas: 1
  selector:
    matchLabels:
      {{- include "deps.selector" (list $root .name) | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "deps.selector" (list $root .name) | nindent 8 }}
    spec:
      {{- with .podSecurityContext }}
      securityContext:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: {{ .name }}
          image: {{ .cfg.image }}
          {{- with .args }}
          args:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          {{- with .env }}
          env:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          ports:
            {{- range $portName, $port := .ports }}
            - name: {{ $portName }}
              containerPort: {{ $port }}
            {{- end }}
          {{- with .probe }}
          readinessProbe:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          resources:
            {{- toYaml .cfg.resources | nindent 12 }}
          volumeMounts:
            - name: data
              mountPath: {{ .mountPath }}
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: [ReadWriteOnce]
        {{- with $root.Values.storageClass }}
        storageClassName: {{ . }}
        {{- end }}
        resources:
          requests:
            storage: {{ .cfg.storage }}
{{- end }}

{{- define "deps.secretEnv" -}}
- name: {{ index . 1 }}
  valueFrom:
    secretKeyRef:
      name: {{ (index . 0).Values.existingSecret }}
      key: {{ index . 1 }}
{{- end }}
