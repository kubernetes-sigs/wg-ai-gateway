package constants

const (
	// AIGatewaySystemNamespace is the namespace where AI Gateway system components are deployed.
	AIGatewaySystemNamespace = "aigateway-system"

	// XDSServerServiceName is the name of the Service that exposes the xDS server.
	XDSServerServiceName = "aigateway-xds-server"

	XDSServerPort = 15001

	EnvoyControllerName = "sigs.k8s.io/wg-ai-gateway-envoy-controller"

	ManagedGatewayLabel = "aigateway.networking.k8s.io/managed"
)

var (
	ControllerNames = []string{EnvoyControllerName}
)
