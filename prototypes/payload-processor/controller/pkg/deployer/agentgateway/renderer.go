package agentgateway

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var (
	//go:embed templates/base.yaml.tpl
	baseTemplate string
)

// baseTemplateParams holds parameters for the base deployment template.
type baseTemplateParams struct {
	ResourceName string
	Namespace    string
	GatewayName  string
	GatewayUID   string
	Image        string
	XDSAddress   string
	Ports        []corev1.ContainerPort
}

// renderBaseTemplate renders the agentgateway deployment and service manifests.
func renderBaseTemplate(gw *gwv1.Gateway, image, xdsAddr string) ([]string, error) {
	name := dataplaneName(gw)

	var ports []corev1.ContainerPort
	for _, listener := range gw.Spec.Listeners {
		ports = append(ports, corev1.ContainerPort{
			Name:          string(listener.Name),
			ContainerPort: int32(listener.Port),
			Protocol:      corev1.ProtocolTCP,
		})
	}

	params := baseTemplateParams{
		ResourceName: name,
		Namespace:    gw.Namespace,
		GatewayName:  gw.Name,
		GatewayUID:   string(gw.UID),
		Image:        image,
		XDSAddress:   xdsAddr,
		Ports:        ports,
	}

	rendered, err := renderTemplate(fmt.Sprintf("agentgateway-base-%s", name), baseTemplate, params)
	if err != nil {
		return nil, err
	}

	return splitYAMLDocument(rendered), nil
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

// splitYAMLDocument splits a multi-document YAML string into individual documents.
func splitYAMLDocument(doc string) []string {
	var result []string
	scanner := bufio.NewScanner(strings.NewReader(doc))
	var current strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}
