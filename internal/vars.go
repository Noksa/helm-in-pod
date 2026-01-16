package internal

import (
	"context"
	"os"

	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/noksa/helm-in-pod/internal/hipns"
	"github.com/noksa/helm-in-pod/internal/hippod"
	"k8s.io/client-go/kubernetes"
)

var (
	Namespace *hipns.Manager
	Pod       *hippod.Manager
)

func init() {
	clientSet := kubernetes.NewForConfigOrDie(operatorkclient.GetClientConfig())
	hostname, _ := os.Hostname()
	ctx := context.Background()

	Namespace = hipns.NewManager(clientSet, ctx)
	Pod = hippod.NewManager(clientSet, ctx, hostname)
}
