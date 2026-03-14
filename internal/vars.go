package internal

import (
	"context"
	"os"

	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/noksa/helm-in-pod/internal/hipns"
	"github.com/noksa/helm-in-pod/internal/hippod"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	namespace *hipns.Manager
	pod       *hippod.Manager
)

// buildConfigOverrides returns clientcmd.ConfigOverrides respecting HELM_KUBECONTEXT.
// Extracted for testability.
func buildConfigOverrides() *clientcmd.ConfigOverrides {
	overrides := &clientcmd.ConfigOverrides{}
	if ctx := os.Getenv("HELM_KUBECONTEXT"); ctx != "" {
		overrides.CurrentContext = ctx
	}
	return overrides
}

func InitManagers() {
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		buildConfigOverrides(),
	)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		panic(err)
	}

	operatorkclient.SetDefaultConfig(config)

	hostname, _ := os.Hostname()
	ctx := context.Background()

	namespace = hipns.NewManager(ctx)
	pod = hippod.NewManager(ctx, hostname)
}

func Namespace() *hipns.Manager {
	return namespace
}

func Pod() *hippod.Manager {
	return pod
}
