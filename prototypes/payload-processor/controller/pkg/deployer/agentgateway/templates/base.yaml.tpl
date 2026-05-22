apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .ResourceName }}
  namespace: {{ .Namespace }}
  labels:
    app: {{ .ResourceName }}
    app.kubernetes.io/managed-by: payload-processor-controller
    gateway.networking.k8s.io/gateway-name: {{ .GatewayName }}
    gateway.networking.k8s.io/gateway-namespace: {{ .Namespace }}
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
        app.kubernetes.io/managed-by: payload-processor-controller
        gateway.networking.k8s.io/gateway-name: {{ .GatewayName }}
        gateway.networking.k8s.io/gateway-namespace: {{ .Namespace }}
    spec:
      containers:
      - name: agentgateway
        image: {{ .Image }}
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: INSTANCE_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: NAMESPACE
          value: {{ .Namespace }}
        - name: GATEWAY
          value: {{ .GatewayName }}
        - name: XDS_ADDRESS
          value: "http://{{ .XDSAddress }}"
        ports:
        {{- range .Ports }}
        - name: {{ .Name | quote }}
          containerPort: {{ .ContainerPort }}
          protocol: TCP
        {{- end }}
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "2000m"
            memory: "1024Mi"
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .ResourceName }}
  namespace: {{ .Namespace }}
  labels:
    app: {{ .ResourceName }}
    app.kubernetes.io/managed-by: payload-processor-controller
    gateway.networking.k8s.io/gateway-name: {{ .GatewayName }}
    gateway.networking.k8s.io/gateway-namespace: {{ .Namespace }}
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
  {{- range .Ports }}
  - name: {{ .Name | quote }}
    port: {{ .ContainerPort }}
    targetPort: {{ .ContainerPort }}
    protocol: TCP
  {{- end }}
