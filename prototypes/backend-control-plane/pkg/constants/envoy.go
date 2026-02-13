package constants

const (
	// Envoy proxy name format
	ProxyNameFormat = "envoy-proxy-%s"
	// EnvoyBootstrapCfgFileName is the name of the Envoy bootstrap configuration file.
	EnvoyBootstrapCfgFileName = "envoy.yaml"

	// ListenerNameFormat is the format string for Envoy listener names, becoming `listener-<port>`.
	ListenerNameFormat = "listener-%d"
	// RouteNameFormat is the format string for Envoy route configuration names, becoming `route-<port>`.
	RouteNameFormat = "route-%d"
	// EnvoyRouteNameFormat is the format string for individual Envoy route names within a RouteConfiguration,
	// becoming `<namespace>-<httproute-name>-rule<rule-index>-match<match-index>`.
	EnvoyRouteNameFormat = "%s-%s-rule%d-match%d"
	// VHostNameFormat is the format string for Envoy virtual host names, becoming `<gateway-name>-vh-<port>-<domain>`.
	VHostNameFormat = "%s-vh-%d-%s"
	// ClusterNameFormat is the format string for Envoy cluster names, becoming `<namespace>-<backend-name>-<port>`.
	ClusterNameFormat = "%s-%s-%d"
)
