{{/*
renderVals – merge template.defaults, class.parameters, build.parameters
*/}}
{{- define "renderVals" -}}
{{- $t := .Values.template.parameters | default dict -}}
{{- $c := .Values.class.parameters    | default dict -}}
{{- $b := .Values.build.parameters    | default dict -}}

{{/* Create final merged dict */}}
{{- $final := deepCopy $t -}}

{{/* Merge class parameters */}}
{{- range $k, $v := $c }}
  {{- $_ := set $final $k $v }}
{{- end -}}

{{/* Merge build parameters */}}
{{- range $k, $v := $b }}
  {{- $_ := set $final $k $v }}
{{- end -}}

{{- $final | toYaml -}}
{{- end }}