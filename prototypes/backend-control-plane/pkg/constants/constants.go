package constants

const (
	// AIGatewaySystemNamespace is the namespace where AI Gateway system components are deployed.
	AIGatewaySystemNamespace = "ai-gateway-system"

	// XDSServerServiceName is the name of the Service that exposes the xDS server.
	XDSServerServiceName = "ai-gateway-controller"

	XDSServerPort = 15001

	EnvoyControllerName = "sigs.k8s.io/wg-ai-gateway-envoy-controller"

	ManagedGatewayLabel = "aigateway.networking.k8s.io/managed"

	// EnvoyImage is the default Envoy proxy image to use.
	EnvoyImage = "envoyproxy/envoy:v1.37-latest"
)

var (
	ControllerNames = []string{EnvoyControllerName}
)
