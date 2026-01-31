apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .ResourceName }}
  namespace: {{ .Namespace }}
  ownerReferences:
  - apiVersion: gateway.networking.k8s.io/v1
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
  name: {{ .ResourceName }}
  namespace: {{ .Namespace }}
  ownerReferences:
  - apiVersion: gateway.networking.k8s.io/v1
    kind: Gateway
    name: {{ .GatewayName }}
    uid: {{ .GatewayUID }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .ResourceName }}
  namespace: {{ .Namespace }}
  ownerReferences:
  - apiVersion: gateway.networking.k8s.io/v1
    kind: Gateway
    name: {{ .GatewayName }}
    uid: {{ .GatewayUID }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .ResourceName }}
  template:
    metadata:
      labels:
        app: {{ .ResourceName }}
    spec:
      serviceAccountName: {{ .ResourceName }}
      containers:
        - name: envoy
          image: {{ .EnvoyImage }}
          command: ["envoy", "-c", "/etc/envoy/{{.EnvoyBootstrapCfgFileName}}", "--log-level", "trace"]
          resources:
            limits:
              cpu: "2000m"
              memory: "1024Mi"
            requests:
              cpu: "100m"
              memory: "128Mi"
          volumeMounts:
            - name: envoy-bootstrap
              mountPath: /etc/envoy
              readOnly: true
      volumes:
        - name: envoy-bootstrap
          configMap:
            name: {{ .ResourceName }}
            items:
              - key: {{.EnvoyBootstrapCfgFileName}}
                path: {{.EnvoyBootstrapCfgFileName}}
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .ResourceName }}
  namespace: {{ .Namespace }}
  ownerReferences:
  - apiVersion: gateway.networking.k8s.io/v1
    kind: Gateway
    name: {{ .GatewayName }}
    uid: {{ .GatewayUID }}
spec:
  type: LoadBalancer
  selector:
    app: {{ .ResourceName }}
  ports:
  {{- range $key, $val := .Ports }}
  - name: {{ $val.Name | quote }}
    port: {{ $val.Port }}
    protocol: TCP
    appProtocol: {{ $val.AppProtocol }}
  {{- end }}
