apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .NodeID }}
  namespace: {{ .Namespace }}
  ownerReferences:
    - apiVersion: v1
      kind: Gateway
      name: {{ .GatewayName }}
      uid: {{ .GatewayUID }}
data:
  {{.EnvoyBootstrapCfgFileName}}: |
    {{ .Bootstrap | indent 4 }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .NodeID }}
  namespace: {{ .Namespace }}
  ownerReferences:
    - apiVersion: v1
      kind: Gateway
      name: {{ .GatewayName }}
      uid: {{ .GatewayUID }}
---
apiVersion: v1
kind: Deployment
metadata:
  name: {{ .NodeID }}
  namespace: {{ .Namespace }}
  ownerReferences:
    - apiVersion: v1
      kind: Gateway
      name: {{ .GatewayName }}
      uid: {{ .GatewayUID }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .NodeID }}
  template:
    metadata:
      labels:
        app: {{ .NodeID }}
    spec:
      serviceAccountName: {{ .NodeID }}
      containers:
        - name: envoy
          image: {{ .EnvoyImage }}\
          command: ["envoy", "-c", "/etc/envoy/{{.EnvoyBootstrapCfgFileName}}", "--log-level", "debug"]
          volumeMounts:
            - name: envoy-bootstrap
              mountPath: /etc/envoy
              readOnly: true
      volumes:
        - name: envoy-bootstrap
          configMap:
            name: {{ .NodeID }}
            items:
              - key: {{.EnvoyBootstrapCfgFileName}}
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .NodeID }}
  namespace: {{ .Namespace }}
  ownerReferences:
    - apiVersion: v1
      kind: Gateway
      name: {{ .GatewayName }}
      uid: {{ .GatewayUID }}
spec:
  selector:
    app: {{ .NodeID }}
  ports:
  {{- range $key, $val := .Ports }}
  - name: {{ $val.Name | quote }}
    port: {{ $val.Port }}
    protocol: TCP
    appProtocol: {{ $val.AppProtocol }}
  {{- end }}
