module sigs.k8s.io/cluster-api-provider-ucloud

go 1.13

replace sigs.k8s.io/cluster-api-provider-ucloud => ../cluster-api-provider-ucloud-new-2

require (
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/google/uuid v1.1.1
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/pkg/errors v0.9.1
	github.com/ucloud/ucloud-sdk-go v0.15.0
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200229041039-0a110f9eb7ab
	sigs.k8s.io/cluster-api v0.3.2
	sigs.k8s.io/controller-runtime v0.5.1
)
