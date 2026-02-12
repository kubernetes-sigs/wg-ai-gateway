package envoy

import (
	"context"
	"fmt"

	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	tlsinspector "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/tls_inspector/v3"
	envoyproxytypes "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"google.golang.org/protobuf/types/known/anypb"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	discoverylisters "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/klog/v2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gatewaylisters "sigs.k8s.io/gateway-api/pkg/client/listers/apis/v1"

	aigatewaylisters "sigs.k8s.io/wg-ai-gateway/prototypes/internal/backend/k8s/client/listers/api/v0alpha0"
	"sigs.k8s.io/wg-ai-gateway/prototypes/backend-control-plane/pkg/constants"
)

// Inspired by https://github.com/kubernetes-sigs/kube-agentic-networking/blob/prototype/pkg/translator/translator.go

type Translator interface {
	TranslateGatewayAndReferencesToXDS(context.Context, *gatewayv1.Gateway) (map[resourcev3.Type][]envoyproxytypes.Resource, map[types.NamespacedName][]gatewayv1.RouteParentStatus, error)
}

type translator struct {
	kubeClient    kubernetes.Interface
	gatewayClient gatewayclientset.Interface

	namespaceLister     corev1listers.NamespaceLister
	serviceLister       corev1listers.ServiceLister
	secretLister        corev1listers.SecretLister
	endpointSliceLister discoverylisters.EndpointSliceLister
	gatewayLister       gatewaylisters.GatewayLister
	httprouteLister     gatewaylisters.HTTPRouteLister
	backendLister       aigatewaylisters.XBackendDestinationLister
}

func New(
	kubeClient kubernetes.Interface,
	gatewayClient gatewayclientset.Interface,
	namespaceLister corev1listers.NamespaceLister,
	serviceLister corev1listers.ServiceLister,
	secretLister corev1listers.SecretLister,
	endpointSliceLister discoverylisters.EndpointSliceLister,
	gatewayLister gatewaylisters.GatewayLister,
	httpRouteLister gatewaylisters.HTTPRouteLister,
	backendLister aigatewaylisters.XBackendDestinationLister,
) Translator {
	return &translator{
		kubeClient:          kubeClient,
		gatewayClient:       gatewayClient,
		namespaceLister:     namespaceLister,
		serviceLister:       serviceLister,
		secretLister:        secretLister,
		endpointSliceLister: endpointSliceLister,
		gatewayLister:       gatewayLister,
		httprouteLister:     httpRouteLister,
		backendLister:       backendLister,
	}
}

var (
	SupportedKinds = sets.New[gatewayv1.Kind](
		"HTTPRoute",
	)
)

func (t *translator) TranslateGatewayAndReferencesToXDS(ctx context.Context, gateway *gatewayv1.Gateway) (map[resourcev3.Type][]envoyproxytypes.Resource, map[types.NamespacedName][]gatewayv1.RouteParentStatus, error) {
	httpRoutesByListener, httpRouteStatuses, err := t.gatherRoutesAndParentStatusesForGateway(ctx, gateway)
	if err != nil {
		return nil, nil, err
	}

	xdsResources, _, err := t.buildXDSFromGatewayAndRoutes(gateway, httpRoutesByListener, httpRouteStatuses)
	if err != nil {
		return nil, nil, err
	}

	return xdsResources, httpRouteStatuses, nil
}

func (t *translator) gatherRoutesAndParentStatusesForGateway(ctx context.Context, gateway *gatewayv1.Gateway) (map[gatewayv1.SectionName][]*gatewayv1.HTTPRoute, map[types.NamespacedName][]gatewayv1.RouteParentStatus, error) {
	httpRouteStatuses := make(map[types.NamespacedName][]gatewayv1.RouteParentStatus)
	routesByListener := make(map[gatewayv1.SectionName][]*gatewayv1.HTTPRoute)
	// 1. List all HTTPRoutes for this Gateway
	// TODO: Support other route kinds
	httpRoutes, err := t.listHTTPRoutesForGateway(ctx, gateway)
	if err != nil {
		return nil, nil, err
	}

	// 2. Validate each route and create an index of listener -> route
	for _, route := range httpRoutes {
		key := types.NamespacedName{Namespace: route.Namespace, Name: route.Name}
		parentStatuses, acceptingListeners := t.validateHTTPRoute(gateway, route)

		if len(parentStatuses) > 0 {
			httpRouteStatuses[key] = parentStatuses
		}

		// If the route was accepted, associate it with the listeners that accepted it.
		if len(acceptingListeners) > 0 {
			// Associate the accepted route with the listeners that will handle it.
			// Use a set to prevent adding a route multiple times to the same listener.
			processedListeners := make(map[gatewayv1.SectionName]bool)
			for _, listener := range acceptingListeners {
				if _, ok := processedListeners[listener.Name]; !ok {
					routesByListener[listener.Name] = append(routesByListener[listener.Name], route)
					processedListeners[listener.Name] = true
				}
			}
		}
	}

	return routesByListener, httpRouteStatuses, nil
}

func (t *translator) listHTTPRoutesForGateway(_ context.Context, gateway *gatewayv1.Gateway) ([]*gatewayv1.HTTPRoute, error) {
	var httpRoutes []*gatewayv1.HTTPRoute
	routeList, err := t.httprouteLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("failed to list HTTPRoutes: %v", err)
		return nil, err
	}

	for _, route := range routeList {
		// check the route's parent references to see if it references the gateway
		for _, parentRef := range route.Spec.ParentRefs {
			refNamespace := route.Namespace
			if parentRef.Namespace != nil {
				refNamespace = string(*parentRef.Namespace)
			}
			if parentRef.Name == gatewayv1.ObjectName(gateway.Name) && refNamespace == gateway.Namespace {
				httpRoutes = append(httpRoutes, route)
				break
			}
		}
	}
	return httpRoutes, nil
}

// validateHTTPRoute is the definitive validation function. It iterates through all
// parentRefs of an HTTPRoute and generates a complete RouteParentStatus for each one
// that targets the specified Gateway. It also returns a slice of all listeners
// that ended up accepting the route.
func (t *translator) validateHTTPRoute(
	gateway *gatewayv1.Gateway,
	httpRoute *gatewayv1.HTTPRoute,
) ([]gatewayv1.RouteParentStatus, []gatewayv1.Listener) {

	var parentStatuses []gatewayv1.RouteParentStatus
	// Use a map to collect a unique set of listeners that accepted the route.
	acceptedListenerSet := make(map[gatewayv1.SectionName]gatewayv1.Listener)

	// --- Determine the ResolvedRefs status for the entire Route first. ---
	// This is a property of the route itself, independent of any parent.
	resolvedRefsCondition := metav1.Condition{
		Type:               string(gatewayv1.RouteConditionResolvedRefs),
		ObservedGeneration: httpRoute.Generation,
		LastTransitionTime: metav1.Now(),
	}

	// --- Iterate over EACH ParentRef in the HTTPRoute ---
	for _, parentRef := range httpRoute.Spec.ParentRefs {
		// We only care about refs that target our current Gateway.
		refNamespace := httpRoute.Namespace
		if parentRef.Namespace != nil {
			refNamespace = string(*parentRef.Namespace)
		}
		if parentRef.Name != gatewayv1.ObjectName(gateway.Name) || refNamespace != gateway.Namespace {
			continue // This ref is for another Gateway.
		}

		// This ref targets our Gateway. We MUST generate a status for it.
		var listenersForThisRef []gatewayv1.Listener
		rejectionReason := gatewayv1.RouteReasonNoMatchingParent

		// --- Find all listeners on the Gateway that match this specific parentRef ---
		for _, listener := range gateway.Spec.Listeners {
			sectionNameMatches := (parentRef.SectionName == nil) || (*parentRef.SectionName == listener.Name)
			portMatches := (parentRef.Port == nil) || (*parentRef.Port == listener.Port)

			if sectionNameMatches && portMatches {
				// The listener matches the ref. Now check if the listener's policy (e.g., hostname) allows it.
				if !isAllowedByListener(gateway, listener, httpRoute, t.namespaceLister) {
					rejectionReason = gatewayv1.RouteReasonNotAllowedByListeners
					continue
				}
				if !isAllowedByHostname(listener, httpRoute) {
					rejectionReason = gatewayv1.RouteReasonNoMatchingListenerHostname
					continue
				}
				listenersForThisRef = append(listenersForThisRef, listener)
			}
		}

		// --- Build the final status for this ParentRef ---
		status := gatewayv1.RouteParentStatus{
			ParentRef:      parentRef,
			ControllerName: constants.EnvoyControllerName,
			Conditions:     []metav1.Condition{},
		}

		// Create the 'Accepted' condition based on the listener validation.
		acceptedCondition := metav1.Condition{
			Type:               string(gatewayv1.RouteConditionAccepted),
			ObservedGeneration: httpRoute.Generation,
			LastTransitionTime: metav1.Now(),
		}

		if len(listenersForThisRef) == 0 {
			acceptedCondition.Status = metav1.ConditionFalse
			acceptedCondition.Reason = string(rejectionReason)
			acceptedCondition.Message = "No listener matched the parentRef."
			if rejectionReason == gatewayv1.RouteReasonNotAllowedByListeners {
				acceptedCondition.Message = "Route is not allowed by a listener's policy."
			} else {
				acceptedCondition.Message = "The route's hostnames do not match any listener hostnames."
			}
		} else {
			acceptedCondition.Status = metav1.ConditionTrue
			acceptedCondition.Reason = string(gatewayv1.RouteReasonAccepted)
			acceptedCondition.Message = "Route is accepted."
			for _, l := range listenersForThisRef {
				acceptedListenerSet[l.Name] = l
			}
		}

		// --- 4. Combine the two independent conditions into the final status. ---
		status.Conditions = append(status.Conditions, acceptedCondition, resolvedRefsCondition)
		parentStatuses = append(parentStatuses, status)
	}

	var allAcceptingListeners []gatewayv1.Listener
	for _, l := range acceptedListenerSet {
		allAcceptingListeners = append(allAcceptingListeners, l)
	}

	return parentStatuses, allAcceptingListeners
}

// buildEDSResources generates EDS resources for Kubernetes Service backends
func (t *translator) buildEDSResources(allBackends []RouteBackend) ([]envoyproxytypes.Resource, error) {
	var edsResources []envoyproxytypes.Resource

	for _, backend := range allBackends {
		// Only generate EDS for Kubernetes Service backends
		if backend.ResolutionType != RouteBackendResolutionTypeEDS {
			continue
		}

		// Determine the ports to create EDS for
		var ports []uint32
		if len(backend.Ports) > 0 {
			for _, port := range backend.Ports {
				ports = append(ports, port.Number)
			}
		} else {
			return nil, fmt.Errorf("backend %s has no ports defined", backend.String())
		}

		for _, port := range ports {
			eds, err := t.generateEDSFromService(backend.ClusterName(), backend.Source.Namespace, port)
			if err != nil {
				klog.Errorf("Failed to generate EDS for backend %s port %d: %v", backend.String(), port, err)
				continue
			}
			edsResources = append(edsResources, eds)
		}
	}

	return edsResources, nil
}

// Start with the gateway and accepted, validated routes and convert them into xDS resources.
func (t *translator) buildXDSFromGatewayAndRoutes(
	gateway *gatewayv1.Gateway,
	routesByListener map[gatewayv1.SectionName][]*gatewayv1.HTTPRoute,
	parentStatuses map[types.NamespacedName][]gatewayv1.RouteParentStatus,
) (map[resourcev3.Type][]envoyproxytypes.Resource, []gatewayv1.ListenerStatus, error) {

	// Start building Envoy config using only the pre-validated and accepted routes
	envoyClusters := make(map[string]envoyproxytypes.Resource)
	allListenerStatuses := make(map[gatewayv1.SectionName]gatewayv1.ListenerStatus)
	var allBackends []RouteBackend

	// First, group each listener by port (there can be multiple listeners on the same port)
	listenersByPort := make(map[int32][]gatewayv1.Listener)
	for _, listener := range gateway.Spec.Listeners {
		listenersByPort[listener.Port] = append(listenersByPort[listener.Port], listener)
	}

	listenerConflictConditions := t.validateListenerConflicts(gateway)

	finalEnvoyListeners := []*listenerv3.Listener{}
	// For each port on the gateway, build an Envoy listener
	for port, listeners := range listenersByPort {
		envoyListener, listenerStatuses, backends, err := t.buildEnvoyListenerForPort(gateway, port, listeners, routesByListener, parentStatuses, allListenerStatuses, listenerConflictConditions, envoyClusters)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build listener for port %d: %w", port, err)
		}
		if envoyListener != nil {
			finalEnvoyListeners = append(finalEnvoyListeners, envoyListener)
		}
		allBackends = append(allBackends, backends...)
		// Update listener statuses
		for _, ls := range listenerStatuses {
			allListenerStatuses[ls.Name] = ls
		}
	}

	// Convert clusters map to slice
	clustersSlice := make([]envoyproxytypes.Resource, 0, len(envoyClusters))
	for _, cluster := range envoyClusters {
		clustersSlice = append(clustersSlice, cluster)
	}

	// Convert listeners to resource slice
	listenerResources := make([]envoyproxytypes.Resource, len(finalEnvoyListeners))
	for i, listener := range finalEnvoyListeners {
		listenerResources[i] = listener
	}

	// Convert listener statuses to ordered slice
	orderedStatuses := make([]gatewayv1.ListenerStatus, len(gateway.Spec.Listeners))
	for i, listener := range gateway.Spec.Listeners {
		orderedStatuses[i] = allListenerStatuses[listener.Name]
	}

	// Generate EDS resources for Kubernetes Service backends
	edsResources, err := t.buildEDSResources(allBackends)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build EDS resources: %w", err)
	}

	return map[resourcev3.Type][]envoyproxytypes.Resource{
		resourcev3.ListenerType: listenerResources,
		resourcev3.ClusterType:  clustersSlice,
		resourcev3.EndpointType: edsResources,
	}, orderedStatuses, nil
}

func (t *translator) buildEnvoyListenerForPort(
	gateway *gatewayv1.Gateway,
	port int32,
	listeners []gatewayv1.Listener,
	routesByListener map[gatewayv1.SectionName][]*gatewayv1.HTTPRoute,
	parentStatuses map[types.NamespacedName][]gatewayv1.RouteParentStatus,
	allListenerStatuses map[gatewayv1.SectionName]gatewayv1.ListenerStatus,
	listenerConflictConditions map[gatewayv1.SectionName][]metav1.Condition,
	envoyClusters map[string]envoyproxytypes.Resource,
) (*listenerv3.Listener, []gatewayv1.ListenerStatus, []RouteBackend, error) {
	var filterChains []*listenerv3.FilterChain
	virtualHostsforPort := make(map[string]*routev3.VirtualHost)
	var listenerStatuses []gatewayv1.ListenerStatus
	var allBackendsForListener []RouteBackend

	// Generate a filter chain for each listener
	for _, listener := range listeners {
		var attachedRoutes int32 // the number of routes attached to this listener
		listenerStatus, isValid := t.validateListener(listener, gateway.Generation, allListenerStatuses, listenerConflictConditions)
		if !isValid {
			listenerStatuses = append(listenerStatuses, *listenerStatus)
			continue // Skip invalid or conflicted listeners
		}

		// Now translate the listener into an Envoy route if the protocol is valid (e.g. HTTP/HTTPS/GRPC)
		switch listener.Protocol {
		case gatewayv1.HTTPProtocolType, gatewayv1.HTTPSProtocolType:
			for _, route := range routesByListener[listener.Name] {
				routes, allValidBackends, resolvedRefsCondition := translateHTTPRouteToEnvoyRoutes(route, t.serviceLister, t.backendLister)

				// Track backends for EDS generation
				allBackendsForListener = append(allBackendsForListener, allValidBackends...)

				// Update the route status with ResolvedRefs condition
				key := types.NamespacedName{Name: route.Name, Namespace: route.Namespace}
				currentParentStatuses := parentStatuses[key]
				for i := range currentParentStatuses {
					// Only add the ResolvedRefs condition if the parent was Accepted.
					if meta.IsStatusConditionTrue(currentParentStatuses[i].Conditions, string(gatewayv1.RouteConditionAccepted)) {
						meta.SetStatusCondition(&currentParentStatuses[i].Conditions, resolvedRefsCondition)
					}
				}
				parentStatuses[key] = currentParentStatuses

				// Build clusters from backends
				clusters, err := t.buildClustersFromBackends(allValidBackends)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("failed to build clusters from HTTPRoute %s/%s: %w", route.Namespace, route.Name, err)
				}
				for _, cluster := range clusters {
					envoyClusters[cluster.Name] = cluster
				}

				// Aggregate Envoy routes into VirtualHosts
				if len(routes) > 0 {
					attachedRoutes++
					// Get the domain for this listener's VirtualHost
					vhostDomains := getIntersectingHostnames(listener, route.Spec.Hostnames)
					for _, domain := range vhostDomains {
						vh, ok := virtualHostsforPort[domain]
						if !ok {
							vh = &routev3.VirtualHost{
								Name:    fmt.Sprintf(constants.VHostNameFormat, gateway.Name, port, domain),
								Domains: []string{domain},
							}
							virtualHostsforPort[domain] = vh
						}
						vh.Routes = append(vh.Routes, routes...)
					}
				}
			}

			// Create filter chain for this listener
			// Build route config first
			allVirtualHosts := make([]*routev3.VirtualHost, 0, len(virtualHostsforPort))
			for _, vh := range virtualHostsforPort {
				// Sort routes by precedence
				sortRoutes(vh.Routes)
				allVirtualHosts = append(allVirtualHosts, vh)
			}

			// Create route configuration
			routeConfig := &routev3.RouteConfiguration{
				Name:         fmt.Sprintf("listener_%s_routes", listener.Name),
				VirtualHosts: allVirtualHosts,
			}

			filterChain, err := t.translateListenerToFilterChain(gateway, listener, routeConfig)
			if err != nil {
				meta.SetStatusCondition(&listenerStatus.Conditions, metav1.Condition{
					Type:               string(gatewayv1.ListenerConditionProgrammed),
					Status:             metav1.ConditionFalse,
					Reason:             string(gatewayv1.ListenerReasonInvalid),
					Message:            fmt.Sprintf("Failed to program listener: %v", err),
					ObservedGeneration: gateway.Generation,
				})
			} else {
				meta.SetStatusCondition(&listenerStatus.Conditions, metav1.Condition{
					Type:               string(gatewayv1.ListenerConditionProgrammed),
					Status:             metav1.ConditionTrue,
					Reason:             string(gatewayv1.ListenerReasonProgrammed),
					Message:            "Listener is programmed",
					ObservedGeneration: gateway.Generation,
				})
				filterChains = append(filterChains, filterChain)
			}

		default:
			klog.Warningf("Unsupported listener protocol %s for routing on Gateway %s", listener.Protocol, types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}.String())
		}

		listenerStatus.AttachedRoutes = attachedRoutes
		meta.SetStatusCondition(&listenerStatus.Conditions, metav1.Condition{
			Type:               string(gatewayv1.ListenerConditionAccepted),
			Status:             metav1.ConditionTrue,
			Reason:             string(gatewayv1.ListenerReasonAccepted),
			Message:            "Listener is valid",
			ObservedGeneration: gateway.Generation,
		})
		allListenerStatuses[listener.Name] = *listenerStatus
		listenerStatuses = append(listenerStatuses, *listenerStatus)
	}

	// Create Envoy Listener if there are any filter chains
	if len(filterChains) > 0 {
		envoyListener := &listenerv3.Listener{
			Name:            fmt.Sprintf(constants.ListenerNameFormat, port),
			Address:         t.createEnvoyAddress(uint32(port)),
			FilterChains:    filterChains,
			ListenerFilters: createListenerFilters(),
		}
		return envoyListener, listenerStatuses, allBackendsForListener, nil
	}

	return nil, listenerStatuses, allBackendsForListener, nil
}

func createListenerFilters() []*listenerv3.ListenerFilter {
	tlsInspectorConfig, _ := anypb.New(&tlsinspector.TlsInspector{})
	return []*listenerv3.ListenerFilter{
		{
			Name: wellknown.TlsInspector,
			ConfigType: &listenerv3.ListenerFilter_TypedConfig{
				TypedConfig: tlsInspectorConfig,
			},
		},
	}
}

// validateListener checks a single listener for conflicts and returns whether it is valid
// and should be processed for xDS translation.
func (t *translator) validateListener(
	listener gatewayv1.Listener,
	observedGeneration int64,
	allListenerStatuses map[gatewayv1.SectionName]gatewayv1.ListenerStatus,
	listenerConflictConditions map[gatewayv1.SectionName][]metav1.Condition,
) (*gatewayv1.ListenerStatus, bool) {
	listenerStatus := gatewayv1.ListenerStatus{
		Name:           gatewayv1.SectionName(listener.Name),
		SupportedKinds: []gatewayv1.RouteGroupKind{},
		Conditions:     listenerConflictConditions[listener.Name],
		AttachedRoutes: 0,
	}
	supportedKinds, allKindsValid := getSupportedKinds(listener)
	listenerStatus.SupportedKinds = supportedKinds

	if !allKindsValid {
		meta.SetStatusCondition(&listenerStatus.Conditions, metav1.Condition{
			Type:               string(gatewayv1.ListenerConditionResolvedRefs),
			Status:             metav1.ConditionFalse,
			Reason:             string(gatewayv1.ListenerReasonInvalidRouteKinds),
			Message:            "Invalid route kinds specified in allowedRoutes",
			ObservedGeneration: observedGeneration,
		})
		allListenerStatuses[listener.Name] = listenerStatus
		return nil, false
	}

	isConflicted := meta.IsStatusConditionTrue(listenerStatus.Conditions, string(gatewayv1.ListenerConditionConflicted))
	// If the listener is conflicted set its status and skip Envoy config generation.
	if isConflicted {
		allListenerStatuses[listener.Name] = listenerStatus
		return nil, false
	}

	// If there are not references issues then set condition to true
	if !meta.IsStatusConditionFalse(listenerStatus.Conditions, string(gatewayv1.ListenerConditionResolvedRefs)) {
		meta.SetStatusCondition(&listenerStatus.Conditions, metav1.Condition{
			Type:               string(gatewayv1.ListenerConditionResolvedRefs),
			Status:             metav1.ConditionTrue,
			Reason:             string(gatewayv1.ListenerReasonResolvedRefs),
			Message:            "All references resolved",
			ObservedGeneration: observedGeneration,
		})
	}

	return &listenerStatus, true
}

func getSupportedKinds(listener gatewayv1.Listener) ([]gatewayv1.RouteGroupKind, bool) {
	supportedKinds := []gatewayv1.RouteGroupKind{}
	allKindsValid := true
	groupName := gatewayv1.Group(gatewayv1.GroupName)

	if listener.AllowedRoutes != nil && len(listener.AllowedRoutes.Kinds) > 0 {
		for _, kind := range listener.AllowedRoutes.Kinds {
			if (kind.Group == nil || *kind.Group == groupName) && SupportedKinds.Has(kind.Kind) {
				supportedKinds = append(supportedKinds, gatewayv1.RouteGroupKind{
					Group: &groupName,
					Kind:  kind.Kind,
				})
			} else {
				allKindsValid = false
			}
		}
	} else if listener.Protocol == gatewayv1.HTTPProtocolType || listener.Protocol == gatewayv1.HTTPSProtocolType {
		for _, kind := range SupportedKinds.UnsortedList() {
			supportedKinds = append(supportedKinds,
				gatewayv1.RouteGroupKind{
					Group: &groupName,
					Kind:  kind,
				},
			)
		}
	}

	return supportedKinds, allKindsValid
}
