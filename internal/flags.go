package internal

type HelmInPodFlags struct {
	Image      string
	Files      string
	FilesAsMap map[string]string
}
