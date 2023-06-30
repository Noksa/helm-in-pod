package internal

import (
	"context"
	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"k8s.io/client-go/kubernetes"
	"os"
)

var (
	clientSet      = kubernetes.NewForConfigOrDie(operatorkclient.GetClientConfig())
	ctx, cancelCtx = context.WithCancel(context.Background())
	Namespace      HelmPodNamespace
	Pod            HelmPod
	myHostname     string
)

func init() {
	hostName, _ := os.Hostname()
	myHostname = hostName
}
