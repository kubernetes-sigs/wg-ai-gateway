package envoy

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/pkg/constants"
)

var (
	//go:embed templates/base.yaml.tpl
	baseTemplate string

	//go:embed templates/bootstrap.yaml.tpl
	bootstrapTemplate string
)

type baseTemplateParams struct {
	NodeID                    string
	ResourceName              string
	Namespace                 string
	GatewayName               string
	GatewayUID                string
	EnvoyBootstrapCfgFileName string
	EnvoyImage                string
	Ports                     []corev1.ServicePort
	Bootstrap                 string
}

type bootstrapTemplateParams struct {
	ID                  string
	Cluster             string
	ControlPlaneAddress string
	ControlPlanePort    uint32
}

func renderBootstrap(cluster, nodeID string) (string, error) {
	params := bootstrapTemplateParams{
		ID:                  nodeID,
		Cluster:             cluster,
		ControlPlaneAddress: fmt.Sprintf("%s.%s.svc.cluster.local", constants.XDSServerServiceName, constants.AIGatewaySystemNamespace),
		ControlPlanePort:    constants.XDSServerPort,
	}

	return renderTemplate(fmt.Sprintf("envoy-bootstrap-%s", nodeID), bootstrapTemplate, params)
}

func renderBaseTemplateForGateway(nodeID string, gateway *gatewayv1.Gateway, image string) ([]string, error) {
	// Generate a descriptive resource name that includes the gateway name
	resourceName := generateResourceName(gateway.Namespace, gateway.Name)

	params := baseTemplateParams{
		NodeID:                    nodeID,
		ResourceName:              resourceName,
		Namespace:                 gateway.Namespace,
		GatewayName:               gateway.Name,
		GatewayUID:                string(gateway.UID),
		EnvoyBootstrapCfgFileName: constants.EnvoyBootstrapCfgFileName,
		EnvoyImage:                image,
		Ports:                     extractServicePorts(*gateway),
	}

	bootstrap, err := renderBootstrap(types.NamespacedName{
		Namespace: gateway.Namespace,
		Name:      gateway.Name,
	}.String(), nodeID)
	if err != nil {
		return nil, err
	}
	params.Bootstrap = bootstrap

	rendered, err := renderTemplate(fmt.Sprintf("envoy-base-%s", nodeID), baseTemplate, params)
	if err != nil {
		return nil, err
	}

	result := splitYAMLDocument(rendered)
	return result, nil
}

func renderTemplate(name, tpl string, params interface{}) (string, error) {
	funcMap := template.FuncMap{
		"indent": func(spaces int, text string) string {
			lines := strings.Split(text, "\n")
			indent := strings.Repeat(" ", spaces)
			for i := range lines {
				if lines[i] != "" {
					lines[i] = indent + lines[i]
				}
			}
			return strings.Join(lines, "\n")
		},
		"quote": func(text string) string {
			return fmt.Sprintf(`"%s"`, text)
		},
	}

	t, err := template.New(name).Funcs(funcMap).Parse(tpl)
	if err != nil {
		return "", fmt.Errorf("error parsing template %s: %w", name, err)
	}

	var buff bytes.Buffer
	if err := t.Execute(&buff, params); err != nil {
		return "", fmt.Errorf("error executing template %s: %w", name, err)
	}

	return buff.String(), nil
}

func extractServicePorts(gw gatewayv1.Gateway) []corev1.ServicePort {
	svcPorts := make([]corev1.ServicePort, 0, len(gw.Spec.Listeners))
	portNums := sets.New[int32]()
	for i, l := range gw.Spec.Listeners {
		if portNums.Has(l.Port) {
			continue
		}
		portNums.Insert(l.Port)
		name := sanitizeListenerNameForPort(string(l.Name))
		if name == "" {
			// Should not happen since name is required, but in case an invalid resource gets in...
			name = fmt.Sprintf("%s-%d", strings.ToLower(string(l.Protocol)), i)
		}
		appProtocol := strings.ToLower(string(l.Protocol))
		svcPorts = append(svcPorts, corev1.ServicePort{
			Name:        name,
			Port:        l.Port,
			AppProtocol: &appProtocol,
		})
	}
	return svcPorts
}

// ListenerName allows periods and 253 chars.
// We map this to service port name which does not allow period and only 63 chars.
func sanitizeListenerNameForPort(s string) string {
	// In theory, this mapping can result in a duplicate, but probably not likely
	s = strings.ReplaceAll(s, ".", "-")
	if len(s) <= 63 {
		return s
	}
	return s[:63]
}

// splitYAMLDocument splits the given yaml doc if it's multipart document.
func splitYAMLDocument(yamlText string) []string {
	out := make([]string, 0)
	reader := bufio.NewReader(strings.NewReader(yamlText))

	parts := []string{}
	active := strings.Builder{}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if line != "" {
				active.WriteString(line)
			}
			break
		}

		if strings.HasPrefix(line, "---") {
			parts = append(parts, active.String())
			active = strings.Builder{}
		} else {
			active.WriteString(line)
		}
	}

	if active.Len() > 0 {
		parts = append(parts, active.String())
	}

	for _, part := range parts {
		part := strings.TrimSpace(part)
		if len(part) > 0 {
			out = append(out, part)
		}
	}
	return out
}
