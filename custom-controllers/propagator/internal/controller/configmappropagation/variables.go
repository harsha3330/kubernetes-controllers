package controller

import "errors"

var defaultSystemNamespaces = map[string]struct{}{
	"kube-system":     {},
	"kube-public":     {},
	"kube-node-lease": {},
}

var (
	FinalizerName       = "sync.propagators.io/finalizer"
	OwnerLabelKey       = "sync.propagators.io/owner"
	OwnerUIDAnnotation  = "sync.propagators.io/owner-uid"
	ManagedByLabelKey   = "sync.propagators.io/managed-by"
	ManagedByLabelValue = "configmap-propagator"
)

var (
	ErrDeletingTargets = errors.New("failed to remove/orphan ConfigMaps of targets")
)
