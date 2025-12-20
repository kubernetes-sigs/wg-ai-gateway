package gvr

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	Backend = schema.GroupVersionResource{Group: "ainetworking.prototype.x-k8s.io", Version: "v0alpha0", Resource: "backends"}
)
