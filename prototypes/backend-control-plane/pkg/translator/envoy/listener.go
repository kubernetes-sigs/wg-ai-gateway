package envoy

import (
	"encoding/pem"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// setListenerCondition is a helper to safely set a condition on a listener's status
// in a map of conditions.
func setListenerCondition(
	conditionsMap map[gatewayv1.SectionName][]metav1.Condition,
	listenerName gatewayv1.SectionName,
	condition metav1.Condition,
) {
	// This "get, modify, set" pattern is the standard way to
	// work around the Go constraint that map values are not addressable.
	conditions := conditionsMap[listenerName]
	if conditions == nil {
		conditions = []metav1.Condition{}
	}
	meta.SetStatusCondition(&conditions, condition)
	conditionsMap[listenerName] = conditions
}

// validateListeners checks for conflicts among all listeners on a Gateway as per the spec.
// It returns a map of conflicted listener conditions and a Gateway-level condition if any conflicts exist.
func (t *translator) validateListenerConflicts(gateway *gatewayv1.Gateway) map[gatewayv1.SectionName][]metav1.Condition {
	listenerConditions := make(map[gatewayv1.SectionName][]metav1.Condition)
	for _, listener := range gateway.Spec.Listeners {
		// Initialize with a fresh slice.
		listenerConditions[listener.Name] = []metav1.Condition{}
	}

	// Check for Port and Hostname Conflicts
	listenersByPort := make(map[gatewayv1.PortNumber][]gatewayv1.Listener)
	for _, listener := range gateway.Spec.Listeners {
		listenersByPort[listener.Port] = append(listenersByPort[listener.Port], listener)
	}

	for _, listenersOnPort := range listenersByPort {
		// Rule: A TCP listener cannot share a port with HTTP/HTTPS/TLS listeners.
		hasTCP := false
		hasHTTPTLS := false
		for _, listener := range listenersOnPort {
			if listener.Protocol == gatewayv1.TCPProtocolType || listener.Protocol == gatewayv1.UDPProtocolType {
				hasTCP = true
			}
			if listener.Protocol == gatewayv1.HTTPProtocolType || listener.Protocol == gatewayv1.HTTPSProtocolType || listener.Protocol == gatewayv1.TLSProtocolType {
				hasHTTPTLS = true
			}
		}

		if hasTCP && hasHTTPTLS {
			for _, listener := range listenersOnPort {
				setListenerCondition(listenerConditions, listener.Name, metav1.Condition{
					Type:    string(gatewayv1.ListenerConditionConflicted),
					Status:  metav1.ConditionTrue,
					Reason:  string(gatewayv1.ListenerReasonProtocolConflict),
					Message: "Protocol conflict: TCP/UDP listeners cannot share a port with HTTP/HTTPS/TLS listeners.",
				})
			}
			continue // Skip further checks for this port
		}

		// Rule: HTTP/HTTPS/TLS listeners on the same port must have unique hostnames.
		seenHostnames := make(map[gatewayv1.Hostname]gatewayv1.SectionName)
		for _, listener := range listenersOnPort {
			// This check only applies to protocols that use hostnames for distinction.
			if listener.Protocol == gatewayv1.HTTPProtocolType || listener.Protocol == gatewayv1.HTTPSProtocolType || listener.Protocol == gatewayv1.TLSProtocolType {
				hostname := gatewayv1.Hostname("")
				if listener.Hostname != nil {
					hostname = *listener.Hostname
				}

				if conflictingListenerName, exists := seenHostnames[hostname]; exists {
					conflictedCondition := metav1.Condition{
						Type:    string(gatewayv1.ListenerConditionConflicted),
						Status:  metav1.ConditionTrue,
						Reason:  string(gatewayv1.ListenerReasonHostnameConflict),
						Message: fmt.Sprintf("Hostname '%s' conflicts with another listener on the same port.", hostname),
					}
					setListenerCondition(listenerConditions, listener.Name, conflictedCondition)
					setListenerCondition(listenerConditions, conflictingListenerName, conflictedCondition)
				} else {
					seenHostnames[hostname] = listener.Name
				}
			}
		}
	}

	for _, listener := range gateway.Spec.Listeners {
		// If a listener is already conflicted, we don't need to check its secrets.
		if meta.IsStatusConditionTrue(listenerConditions[listener.Name], string(gatewayv1.ListenerConditionConflicted)) {
			continue
		}

		if listener.TLS == nil {
			// No TLS config, so no secrets to resolve. This listener is considered resolved.
			setListenerCondition(listenerConditions, listener.Name, metav1.Condition{
				Type:    string(gatewayv1.ListenerConditionResolvedRefs),
				Status:  metav1.ConditionTrue,
				Reason:  string(gatewayv1.ListenerReasonResolvedRefs),
				Message: "All references resolved",
			})
			continue
		}

		for _, certRef := range listener.TLS.CertificateRefs {
			if certRef.Group != nil && *certRef.Group != "" {
				setListenerCondition(listenerConditions, listener.Name, metav1.Condition{
					Type:    string(gatewayv1.ListenerConditionResolvedRefs),
					Status:  metav1.ConditionFalse,
					Reason:  string(gatewayv1.ListenerReasonInvalidCertificateRef),
					Message: fmt.Sprintf("unsupported certificate ref grup: %s", *certRef.Group),
				})
				break
			}

			if certRef.Kind != nil && *certRef.Kind != "Secret" {
				setListenerCondition(listenerConditions, listener.Name, metav1.Condition{
					Type:    string(gatewayv1.ListenerConditionResolvedRefs),
					Status:  metav1.ConditionFalse,
					Reason:  string(gatewayv1.ListenerReasonInvalidCertificateRef),
					Message: fmt.Sprintf("unsupported certificate ref kind: %s", *certRef.Kind),
				})
				break
			}

			secretNamespace := gateway.Namespace
			if certRef.Namespace != nil {
				secretNamespace = string(*certRef.Namespace)
			}

			secret, err := t.secretLister.Secrets(secretNamespace).Get(string(certRef.Name))
			if err != nil {
				setListenerCondition(listenerConditions, listener.Name, metav1.Condition{
					Type:    string(gatewayv1.ListenerConditionResolvedRefs),
					Status:  metav1.ConditionFalse,
					Reason:  string(gatewayv1.ListenerReasonInvalidCertificateRef),
					Message: fmt.Sprintf("reference to Secret %s/%s not found", secretNamespace, certRef.Name),
				})
				break
			}
			if err := validateSecretCertificate(secret); err != nil {
				setListenerCondition(listenerConditions, listener.Name, metav1.Condition{
					Type:    string(gatewayv1.ListenerConditionResolvedRefs),
					Status:  metav1.ConditionFalse,
					Reason:  string(gatewayv1.ListenerReasonInvalidCertificateRef),
					Message: fmt.Sprintf("malformed Secret %s/%s : %v", secretNamespace, certRef.Name, err.Error()),
				})
				break
			}
		}

		// Set the ResolvedRefs condition based on the outcome of the secret validation.
		if !meta.IsStatusConditionFalse(listenerConditions[listener.Name], string(gatewayv1.ListenerConditionResolvedRefs)) {
			setListenerCondition(listenerConditions, listener.Name, metav1.Condition{
				Type:    string(gatewayv1.ListenerConditionResolvedRefs),
				Status:  metav1.ConditionTrue,
				Reason:  string(gatewayv1.ListenerReasonResolvedRefs),
				Message: "All references resolved",
			})
		}
	}

	return listenerConditions
}

func validateSecretCertificate(secret *corev1.Secret) error {
	privateKey, ok := secret.Data[corev1.TLSPrivateKeyKey]
	if !ok {
		return fmt.Errorf("secret %s/%s does not contain key %s", secret.Namespace, secret.Name, corev1.TLSPrivateKeyKey)
	}
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return fmt.Errorf("secret %s/%s key %s does not contain a valid PEM-encoded private key", secret.Namespace, secret.Name, corev1.TLSPrivateKeyKey)
	}

	certChain, ok := secret.Data[corev1.TLSCertKey]
	if !ok {
		return fmt.Errorf("secret %s/%s does not contain key %s", secret.Namespace, secret.Name, corev1.TLSCertKey)
	}
	block, _ = pem.Decode(certChain)
	if block == nil {
		return fmt.Errorf("secret %s/%s key %s does not contain a valid PEM-encoded certificate chain", secret.Namespace, secret.Name, corev1.TLSCertKey)
	}
	return nil
}
