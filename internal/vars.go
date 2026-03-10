package internal

import (
	"context"
	"os"

	"github.com/noksa/helm-in-pod/internal/hipns"
	"github.com/noksa/helm-in-pod/internal/hippod"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	Namespace *hipns.Manager
	Pod       *hippod.Manager
)

func getClientConfig() *rest.Config {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}

	if ctx := os.Getenv("HELM_KUBECONTEXT"); ctx != "" {
		configOverrides.CurrentContext = ctx
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		panic(err)
	}
	return config
}

func init() {
	clientSet := kubernetes.NewForConfigOrDie(getClientConfig())
	hostname, _ := os.Hostname()
	ctx := context.Background()

	Namespace = hipns.NewManager(clientSet, ctx)
	Pod = hippod.NewManager(clientSet, ctx, hostname)
}
