package envoy

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/types/known/wrapperspb"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	aigatewayv0alpha0 "sigs.k8s.io/wg-ai-gateway/api/v0alpha0"
	aigatewaylisters "sigs.k8s.io/wg-ai-gateway/k8s/client/listers/api/v0alpha0"
	"sigs.k8s.io/wg-ai-gateway/pkg/constants"
)

// ControllerError represents a structured error that can be used to set failure conditions
type ControllerError struct {
	Reason  string
	Message string
}

func (e *ControllerError) Error() string {
	return e.Message
}
func translateHTTPRouteToEnvoyRoutes(
	httpRoute *gatewayv1.HTTPRoute,
	serviceLister corev1listers.ServiceLister,
	backendLister aigatewaylisters.BackendLister,
) ([]*routev3.Route, []*aigatewayv0alpha0.Backend, metav1.Condition) {
	var envoyRoutes []*routev3.Route
	var allValidBackends []*aigatewayv0alpha0.Backend
	overallCondition := createSuccessCondition(httpRoute.Generation)

	for ruleIndex, rule := range httpRoute.Spec.Rules {
		// These are the different operations that an HTTPRoute rule can specify
		var redirectAction *routev3.RedirectAction
		var requestHeadersToAdd []*corev3.HeaderValueOption
		var requestHeadersToRemove []string
		var responseHeadersToAdd []*corev3.HeaderValueOption
		var responseHeadersToRemove []string
		var urlRewriteAction *routev3.RouteAction

		// Process filters using a switch and delegate logic to helpers.
	FilterLoop:
		for _, filter := range rule.Filters {
			switch filter.Type {
			case gatewayv1.HTTPRouteFilterRequestRedirect:
				redirectAction = translateRequestRedirectFilter(filter.RequestRedirect)
				if redirectAction != nil {
					// Only one redirect filter is allowed per rule: stop processing further filters.
					break FilterLoop
				}
			case gatewayv1.HTTPRouteFilterRequestHeaderModifier:
				adds, removes := translateRequestHeaderModifierFilter(filter.RequestHeaderModifier)
				requestHeadersToAdd = append(requestHeadersToAdd, adds...)
				requestHeadersToRemove = append(requestHeadersToRemove, removes...)
			case gatewayv1.HTTPRouteFilterResponseHeaderModifier:
				adds, removes := translateResponseHeaderModifierFilter(filter.ResponseHeaderModifier)
				responseHeadersToAdd = append(responseHeadersToAdd, adds...)
				responseHeadersToRemove = append(responseHeadersToRemove, removes...)
			case gatewayv1.HTTPRouteFilterURLRewrite:
				urlRewriteAction = translateURLRewriteFilter(filter.URLRewrite)
			case gatewayv1.HTTPRouteFilterExtensionRef:
				// Extension filters are implementation-specific and would need custom handling
				klog.Infof("ExtensionRef filter not implemented: %v", filter.ExtensionRef)
			default:
				// Unsupported filter type; skip
				klog.Warningf("Unsupported HTTPRoute filter type: %s", filter.Type)
			}
		}

		buildRoutesForRule := func(match gatewayv1.HTTPRouteMatch, matchIndex int) {
			routeMatch, matchCondition := translateHTTPRouteMatch(match, httpRoute.Generation)
			if matchCondition.Status == metav1.ConditionFalse {
				overallCondition = matchCondition
				return
			}

			envoyRoute := &routev3.Route{
				Name:                       fmt.Sprintf(constants.EnvoyRouteNameFormat, httpRoute.Namespace, httpRoute.Name, ruleIndex, matchIndex),
				Match:                      routeMatch,
				RequestHeadersToAdd:        requestHeadersToAdd,
				RequestHeadersToRemove:     requestHeadersToRemove,
				ResponseHeadersToAdd:       responseHeadersToAdd,
				ResponseHeadersToRemove:    responseHeadersToRemove,
			}

			if redirectAction != nil {
				// If this is a redirect, set the Redirect action. No backends are needed.
				envoyRoute.Action = &routev3.Route_Redirect{
					Redirect: redirectAction,
				}
			} else {
				// Build the forwarding action with backend clusters
				routeAction, validBackends, err := buildHTTPRouteAction(
					httpRoute.Namespace,
					rule.BackendRefs,
					serviceLister,
					backendLister,
				)
				var controllerErr *ControllerError
				if errors.As(err, &controllerErr) {
					overallCondition = createFailureCondition(gatewayv1.RouteConditionReason(controllerErr.Reason), controllerErr.Message, httpRoute.Generation)
					envoyRoute.Action = &routev3.Route_DirectResponse{
						DirectResponse: &routev3.DirectResponseAction{Status: 500},
					}
					// Skip further processing for this route if backends are invalid.
					envoyRoutes = append(envoyRoutes, envoyRoute)
					return
				}
				allValidBackends = append(allValidBackends, validBackends...)

				// If a URLRewrite filter was present, merge its properties into the RouteAction.
				if urlRewriteAction != nil {
					routeAction.HostRewriteSpecifier = urlRewriteAction.HostRewriteSpecifier
					routeAction.RegexRewrite = urlRewriteAction.RegexRewrite
					routeAction.PrefixRewrite = urlRewriteAction.PrefixRewrite
				}

				envoyRoute.Action = &routev3.Route_Route{
					Route: routeAction,
				}
			}
			envoyRoutes = append(envoyRoutes, envoyRoute)
		}

		if len(rule.Matches) == 0 {
			buildRoutesForRule(gatewayv1.HTTPRouteMatch{}, 0)
		} else {
			for matchIndex, match := range rule.Matches {
				buildRoutesForRule(match, matchIndex)
			}
		}
	}

	// Sort routes by Gateway API precedence rules
	sortRoutes(envoyRoutes)

	return envoyRoutes, allValidBackends, overallCondition
}

func translateRequestRedirectFilter(requestRedirect *gatewayv1.HTTPRequestRedirectFilter) *routev3.RedirectAction {
	if requestRedirect == nil {
		return nil
	}

	redirectAction := &routev3.RedirectAction{}

	// Handle scheme redirect
	if requestRedirect.Scheme != nil {
		redirectAction.SchemeRewriteSpecifier = &routev3.RedirectAction_SchemeRedirect{
			SchemeRedirect: *requestRedirect.Scheme,
		}
	}

	// Handle hostname redirect
	if requestRedirect.Hostname != nil {
		redirectAction.HostRedirect = string(*requestRedirect.Hostname)
	}

	// Handle path redirect
	if requestRedirect.Path != nil {
		switch requestRedirect.Path.Type {
		case gatewayv1.FullPathHTTPPathModifier:
			redirectAction.PathRewriteSpecifier = &routev3.RedirectAction_PathRedirect{
				PathRedirect: *requestRedirect.Path.ReplaceFullPath,
			}
		case gatewayv1.PrefixMatchHTTPPathModifier:
			redirectAction.PathRewriteSpecifier = &routev3.RedirectAction_PrefixRewrite{
				PrefixRewrite: *requestRedirect.Path.ReplacePrefixMatch,
			}
		}
	}

	// Handle port redirect
	if requestRedirect.Port != nil {
		redirectAction.PortRedirect = uint32(*requestRedirect.Port)
	}

	// Handle status code (default to 302 if not specified)
	if requestRedirect.StatusCode != nil {
		redirectAction.ResponseCode = routev3.RedirectAction_RedirectResponseCode(*requestRedirect.StatusCode)
	} else {
		redirectAction.ResponseCode = routev3.RedirectAction_FOUND // 302
	}

	return redirectAction
}

func translateURLRewriteFilter(urlRewrite *gatewayv1.HTTPURLRewriteFilter) *routev3.RouteAction {
	if urlRewrite == nil {
		return nil
	}

	routeAction := &routev3.RouteAction{}
	// The flag prevents the function from returning an empty &routev3.RouteAction{}
	// struct when no actual rewrite is needed.
	rewriteActionSet := false

	// Handle hostname rewrite
	if urlRewrite.Hostname != nil {
		routeAction.HostRewriteSpecifier = &routev3.RouteAction_HostRewriteLiteral{
			HostRewriteLiteral: string(*urlRewrite.Hostname),
		}
	}

	// Handle path rewrite
	if urlRewrite.Path != nil {
		switch urlRewrite.Path.Type {
		case gatewayv1.FullPathHTTPPathModifier:
			if urlRewrite.Path.ReplaceFullPath != nil {
				routeAction.RegexRewrite = &matcherv3.RegexMatchAndSubstitute{
					Pattern:      &matcherv3.RegexMatcher{EngineType: &matcherv3.RegexMatcher_GoogleRe2{}, Regex: ".*"},
					Substitution: *urlRewrite.Path.ReplaceFullPath,
				}
				rewriteActionSet = true
			}
		case gatewayv1.PrefixMatchHTTPPathModifier:
			if urlRewrite.Path.ReplacePrefixMatch != nil {
				routeAction.PrefixRewrite = *urlRewrite.Path.ReplacePrefixMatch
				rewriteActionSet = true
			}
		}
	}

	// If no rewrite actions were set, return nil.
	if !rewriteActionSet {
		return nil
	}

	return routeAction
}

func translateRequestHeaderModifierFilter(headerModifier *gatewayv1.HTTPHeaderFilter) ([]*corev3.HeaderValueOption, []string) {
	if headerModifier == nil {
		return nil, nil
	}

	var headersToAdd []*corev3.HeaderValueOption
	var headersToRemove []string

	// Handle headers to set/add
	if headerModifier.Set != nil {
		for _, header := range headerModifier.Set {
			headersToAdd = append(headersToAdd, &corev3.HeaderValueOption{
				Header: &corev3.HeaderValue{
					Key:   string(header.Name),
					Value: header.Value,
				},
				AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			})
		}
	}

	if headerModifier.Add != nil {
		for _, header := range headerModifier.Add {
			headersToAdd = append(headersToAdd, &corev3.HeaderValueOption{
				Header: &corev3.HeaderValue{
					Key:   string(header.Name),
					Value: header.Value,
				},
				AppendAction: corev3.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD,
			})
		}
	}

	// Handle headers to remove
	if headerModifier.Remove != nil {
		headersToRemove = append(headersToRemove, headerModifier.Remove...)
	}

	return headersToAdd, headersToRemove
}

func translateResponseHeaderModifierFilter(headerModifier *gatewayv1.HTTPHeaderFilter) ([]*corev3.HeaderValueOption, []string) {
	if headerModifier == nil {
		return nil, nil
	}

	var headersToAdd []*corev3.HeaderValueOption
	var headersToRemove []string

	// Handle headers to set/add (same logic as request headers)
	if headerModifier.Set != nil {
		for _, header := range headerModifier.Set {
			headersToAdd = append(headersToAdd, &corev3.HeaderValueOption{
				Header: &corev3.HeaderValue{
					Key:   string(header.Name),
					Value: header.Value,
				},
				AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			})
		}
	}

	if headerModifier.Add != nil {
		for _, header := range headerModifier.Add {
			headersToAdd = append(headersToAdd, &corev3.HeaderValueOption{
				Header: &corev3.HeaderValue{
					Key:   string(header.Name),
					Value: header.Value,
				},
				AppendAction: corev3.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD,
			})
		}
	}

	// Handle headers to remove
	if headerModifier.Remove != nil {
		headersToRemove = append(headersToRemove, headerModifier.Remove...)
	}

	return headersToAdd, headersToRemove
}

func createSuccessCondition(generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               string(gatewayv1.RouteConditionResolvedRefs),
		Status:             metav1.ConditionTrue,
		Reason:             string(gatewayv1.RouteReasonResolvedRefs),
		Message:            "All references resolved",
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	}
}

func createFailureCondition(reason gatewayv1.RouteConditionReason, message string, generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               string(gatewayv1.RouteConditionResolvedRefs),
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	}
}

// buildHTTPRouteAction returns an action, a list of valid BackendRefs, and a structured error.
func buildHTTPRouteAction(
	namespace string,
	backendRefs []gatewayv1.HTTPBackendRef,
	serviceLister corev1listers.ServiceLister,
	backendLister aigatewaylisters.BackendLister,
) (*routev3.RouteAction, []*aigatewayv0alpha0.Backend, error) {
	weightedClusters := &routev3.WeightedCluster{}
	var validBackends []*aigatewayv0alpha0.Backend

	for _, httpBackendRef := range backendRefs {
		backend, err := fetchBackend(namespace, httpBackendRef.BackendRef, backendLister, serviceLister)
		if err != nil {
			return nil, nil, err
		}
		validBackends = append(validBackends, backend)
		weight := int32(1)
		if httpBackendRef.Weight != nil {
			weight = *httpBackendRef.Weight
		}
		if weight == 0 {
			continue
		}

		clusterWeight := &routev3.WeightedCluster_ClusterWeight{
			Name:   fmt.Sprintf(constants.ClusterNameFormat, backend.Namespace, backend.Name),
			Weight: &wrapperspb.UInt32Value{Value: uint32(weight)},
		}

		// Handle hostname rewriting for FQDN backends
		if backend.Spec.Destination.FQDN != nil {
			clusterWeight.HostRewriteSpecifier = &routev3.WeightedCluster_ClusterWeight_HostRewriteLiteral{
				HostRewriteLiteral: backend.Spec.Destination.FQDN.Hostname,
			}
		}

		weightedClusters.Clusters = append(weightedClusters.Clusters, clusterWeight)
	}

	if len(weightedClusters.Clusters) == 0 {
		return nil, nil, &ControllerError{
			Reason:  string(gatewayv1.RouteReasonUnsupportedValue),
			Message: "no valid backends provided with a weight > 0",
		}
	}

	action := &routev3.RouteAction{
		ClusterSpecifier: &routev3.RouteAction_WeightedClusters{
			WeightedClusters: weightedClusters,
		},
	}

	return action, validBackends, nil
}

// fetchBackend retrieves a Backend resource based on the BackendRef
func fetchBackend(
	namespace string,
	backendRef gatewayv1.BackendRef,
	backendLister aigatewaylisters.BackendLister,
	serviceLister corev1listers.ServiceLister,
) (*aigatewayv0alpha0.Backend, error) {
	// Determine the namespace for the backend
	backendNamespace := namespace
	if backendRef.Namespace != nil {
		backendNamespace = string(*backendRef.Namespace)
	}

	// Handle different backend kinds
	switch *backendRef.Kind {
	case "Backend":
		// Fetch the Backend resource
		backend, err := backendLister.Backends(backendNamespace).Get(string(backendRef.Name))
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, &ControllerError{
					Reason:  string(gatewayv1.RouteReasonBackendNotFound),
					Message: fmt.Sprintf("Backend %s/%s not found", backendNamespace, backendRef.Name),
				}
			}
			return nil, err
		}
		return backend, nil

	case "Service":
		// For Service backends, we need to validate the service exists and create a synthetic Backend
		svc, err := serviceLister.Services(backendNamespace).Get(string(backendRef.Name))
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, &ControllerError{
					Reason:  string(gatewayv1.RouteReasonBackendNotFound),
					Message: fmt.Sprintf("Service %s/%s not found", backendNamespace, backendRef.Name),
				}
			}
			return nil, err
		}

		// Create a synthetic Backend for the Service
		return createSyntheticBackendFromService(svc, backendRef.Port), nil

	default:
		return nil, &ControllerError{
			Reason:  string(gatewayv1.RouteReasonUnsupportedValue),
			Message: fmt.Sprintf("unsupported backend kind: %s", *backendRef.Kind),
		}
	}
}

// createSyntheticBackendFromService creates a Backend resource representation from a Kubernetes Service
func createSyntheticBackendFromService(svc *corev1.Service, port *gatewayv1.PortNumber) *aigatewayv0alpha0.Backend {
	// This creates a synthetic Backend that represents the Kubernetes Service
	// In a real implementation, you might want to cache these or handle them differently
	backend := &aigatewayv0alpha0.Backend{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			UID:       types.UID(fmt.Sprintf("synthetic-%s-%s", svc.Namespace, svc.Name)),
		},
		Spec: aigatewayv0alpha0.BackendSpec{
			Destination: aigatewayv0alpha0.BackendDestination{
				Type: aigatewayv0alpha0.BackendTypeKubernetesService,
				// For a Service backend, we don't populate FQDN since it's cluster-internal
			},
		},
	}

	// If a specific port is requested, add it to the backend spec
	if port != nil {
		// Find the corresponding service port
		for _, svcPort := range svc.Spec.Ports {
			if svcPort.Port == int32(*port) {
				backend.Spec.Destination.Ports = []aigatewayv0alpha0.BackendPort{
					{
						Number:   uint32(svcPort.Port),
						Protocol: aigatewayv0alpha0.BackendProtocolHTTP, // Default to HTTP
					},
				}
				break
			}
		}
	}

	return backend
}

// translateHTTPRouteMatch translates a Gateway API HTTPRouteMatch into an Envoy RouteMatch.
func translateHTTPRouteMatch(match gatewayv1.HTTPRouteMatch, generation int64) (*routev3.RouteMatch, metav1.Condition) {
	routeMatch := &routev3.RouteMatch{}

	if match.Path != nil {
		pathType := gatewayv1.PathMatchPathPrefix
		if match.Path.Type != nil {
			pathType = *match.Path.Type
		}
		if match.Path.Value == nil {
			msg := "path match value cannot be nil"
			return nil, createFailureCondition(gatewayv1.RouteReasonUnsupportedValue, msg, generation)
		}
		pathValue := *match.Path.Value

		switch pathType {
		case gatewayv1.PathMatchExact:
			routeMatch.PathSpecifier = &routev3.RouteMatch_Path{Path: pathValue}
		case gatewayv1.PathMatchPathPrefix:
			if pathValue == "/" {
				routeMatch.PathSpecifier = &routev3.RouteMatch_Prefix{Prefix: "/"}
			} else {
				path := strings.TrimSuffix(pathValue, "/")
				routeMatch.PathSpecifier = &routev3.RouteMatch_PathSeparatedPrefix{PathSeparatedPrefix: path}
			}
		case gatewayv1.PathMatchRegularExpression:
			routeMatch.PathSpecifier = &routev3.RouteMatch_SafeRegex{
				SafeRegex: &matcherv3.RegexMatcher{
					EngineType: &matcherv3.RegexMatcher_GoogleRe2{GoogleRe2: &matcherv3.RegexMatcher_GoogleRE2{}},
					Regex:      pathValue,
				},
			}
		default:
			msg := fmt.Sprintf("unsupported path match type: %s", pathType)
			return nil, createFailureCondition(gatewayv1.RouteReasonUnsupportedValue, msg, generation)
		}
	} else {
		// As per Gateway API spec, a nil path match defaults to matching everything.
		routeMatch.PathSpecifier = &routev3.RouteMatch_Prefix{Prefix: "/"}
	}

	// Translate Header Matches
	for _, headerMatch := range match.Headers {
		headerMatcher := &routev3.HeaderMatcher{
			Name: string(headerMatch.Name),
		}
		matchType := gatewayv1.HeaderMatchExact
		if headerMatch.Type != nil {
			matchType = *headerMatch.Type
		}

		switch matchType {
		case gatewayv1.HeaderMatchExact:
			headerMatcher.HeaderMatchSpecifier = &routev3.HeaderMatcher_StringMatch{
				StringMatch: &matcherv3.StringMatcher{
					MatchPattern: &matcherv3.StringMatcher_Exact{Exact: headerMatch.Value},
				},
			}
		case gatewayv1.HeaderMatchRegularExpression:
			headerMatcher.HeaderMatchSpecifier = &routev3.HeaderMatcher_SafeRegexMatch{
				SafeRegexMatch: &matcherv3.RegexMatcher{
					EngineType: &matcherv3.RegexMatcher_GoogleRe2{GoogleRe2: &matcherv3.RegexMatcher_GoogleRE2{}},
					Regex:      headerMatch.Value,
				},
			}
		default:
			msg := fmt.Sprintf("unsupported header match type: %s", matchType)
			return nil, createFailureCondition(gatewayv1.RouteReasonUnsupportedValue, msg, generation)
		}
		routeMatch.Headers = append(routeMatch.Headers, headerMatcher)
	}

	// Translate Query Parameter Matches
	for _, queryMatch := range match.QueryParams {
		// Gateway API only supports "Exact" match for query parameters.
		queryMatcher := &routev3.QueryParameterMatcher{
			Name: string(queryMatch.Name),
			QueryParameterMatchSpecifier: &routev3.QueryParameterMatcher_StringMatch{
				StringMatch: &matcherv3.StringMatcher{
					MatchPattern: &matcherv3.StringMatcher_Exact{Exact: queryMatch.Value},
				},
			},
		}
		routeMatch.QueryParameters = append(routeMatch.QueryParameters, queryMatcher)
	}

	// If all translations were successful, return the final object and a success condition.
	return routeMatch, createSuccessCondition(generation)
}

// sortRoutes is the definitive sorter for Envoy routes based on Gateway API precedence.
func sortRoutes(routes []*routev3.Route) {
	sort.Slice(routes, func(i, j int) bool {
		matchI := routes[i].GetMatch()
		matchJ := routes[j].GetMatch()

		// De-prioritize the catch-all route, ensuring it's always last.
		isCatchAllI := isCatchAll(matchI)
		isCatchAllJ := isCatchAll(matchJ)

		if isCatchAllI != isCatchAllJ {
			// If I is the catch-all, it should come after J (return false).
			// If J is the catch-all, it should come after I (return true).
			return isCatchAllJ
		}

		// Precedence Rule 1: Exact Path Match vs. Other Path Matches
		isExactPathI := matchI.GetPath() != ""
		isExactPathJ := matchJ.GetPath() != ""
		if isExactPathI != isExactPathJ {
			return isExactPathI // Exact path is higher precedence
		}

		// Precedence Rule 2: Longest Prefix Match
		prefixI := getPathMatchValue(matchI)
		prefixJ := getPathMatchValue(matchJ)

		if len(prefixI) != len(prefixJ) {
			return len(prefixI) > len(prefixJ) // Longer prefix is higher precedence
		}

		// Precedence Rule 3: Number of Header Matches
		headerCountI := len(matchI.GetHeaders())
		headerCountJ := len(matchJ.GetHeaders())
		if headerCountI != headerCountJ {
			return headerCountI > headerCountJ // More headers is higher precedence
		}

		// Precedence Rule 4: Number of Query Param Matches
		queryCountI := len(matchI.GetQueryParameters())
		queryCountJ := len(matchJ.GetQueryParameters())
		if queryCountI != queryCountJ {
			return queryCountI > queryCountJ // More query params is higher precedence
		}

		// If all else is equal, maintain original order (stable sort)
		return false
	})
}

// getPathMatchValue is a helper to extract the path string for comparison.
func getPathMatchValue(match *routev3.RouteMatch) string {
	if match.GetPath() != "" {
		return match.GetPath()
	}
	if match.GetPrefix() != "" {
		return match.GetPrefix()
	}
	if match.GetPathSeparatedPrefix() != "" {
		return match.GetPathSeparatedPrefix()
	}
	if sr := match.GetSafeRegex(); sr != nil {
		// Regex Match (used for other PathPrefix)
		// This correctly handles the output of translateHTTPRouteMatch.
		regex := sr.GetRegex()
		// Remove the trailing regex that matches subpaths.
		path := strings.TrimSuffix(regex, "(/.*)?")
		// Remove the quoting added by regexp.QuoteMeta.
		path = strings.ReplaceAll(path, `\`, "")
		return path
	}
	return ""
}

// isCatchAll determines if a route match is a generic "catch-all" rule.
// A catch-all matches all paths ("/") and has no other specific conditions.
func isCatchAll(match *routev3.RouteMatch) bool {
	if match == nil {
		return false
	}
	// It's a catch-all if the path match is for "/" AND there are no other constraints.
	isRootPrefix := match.GetPrefix() == "/"
	hasNoHeaders := len(match.GetHeaders()) == 0
	hasNoParams := len(match.GetQueryParameters()) == 0
	return isRootPrefix && hasNoHeaders && hasNoParams
}
