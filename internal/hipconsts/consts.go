package hipconsts

// Namespace is the single source of truth for the Kubernetes namespace used
// by the plugin. Set at startup from HELM_IN_POD_NAMESPACE; defaults to
// "helm-in-pod". Both hippod and hipns read this var directly.
var Namespace = "helm-in-pod"

const (
	AnnotationHomeDirectory      = "helm-in-pod/home-directory"
	AnnotationHelmFound          = "helm-in-pod/helm-found"
	AnnotationHelm4              = "helm-in-pod/helm4"
	AnnotationLastRepoUpdateTime = "helm-in-pod/last-repo-update-time"

	EnvDaemonName = "HELM_IN_POD_DAEMON_NAME"
	EnvImage      = "HELM_IN_POD_IMAGE"
	EnvNamespace  = "HELM_IN_POD_NAMESPACE"

	LabelOperationID = "helm-in-pod/operation-id"
	LabelManagedBy   = "app.kubernetes.io/managed-by"
	LabelKept        = "helm-in-pod/kept"

	// Sentinel files for copy-from flow
	CopyFromDoneFile = "/tmp/copy-done"

	// Marker written to stdout by the pod script when command finishes (copy-from mode)
	CopyFromExitCodeMarkerPrefix = "###HIP_EXIT_CODE:"
	CopyFromExitCodeMarkerSuffix = "###"

	// Environment variable to enable copy-from wait mode in the pod script
	EnvWaitCopyDone = "WAIT_COPY_DONE"

	// WrappedScriptPath is the fixed path inside the pod for the user command script.
	WrappedScriptPath = "/tmp/hip-wrapped-script.sh"

	// StagedScriptPath is where the wrapped script is placed during bundle copy,
	// before being moved to WrappedScriptPath to trigger execution.
	StagedScriptPath = "/tmp/hip-staged-script.sh"

	// StagedRepoConfigPath is a temporary path where repositories.yaml is placed
	// during bundle copy, before being moved to $HOME/.config/helm/ by the boot command.
	StagedRepoConfigPath = "/tmp/hip-repositories.yaml"
)
