{{/*
Ingress chart labels
*/}}
{{- define "ingress.labels" -}}
app.kubernetes.io/name: ingress
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
