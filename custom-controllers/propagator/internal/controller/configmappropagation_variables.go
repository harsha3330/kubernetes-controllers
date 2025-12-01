package controller

var defaultSystemNamespaces = map[string]struct{}{
	"kube-system":     {},
	"kube-public":     {},
	"kube-node-lease": {},
}

var (
	FinalizerName      = "sync.propagators.io/finalizer"
	OwnerLabelKey      = "sync.propagators.io/owner"
	OwnerUIDAnnotation = "sync.propagators.io/owner-uid"
)
