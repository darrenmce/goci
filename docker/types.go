package docker

type BuildVolume struct {
	Source string
	Target string
}

type BuildContainer struct {
	Name    string
	Image   string
	Volumes []BuildVolume
	WorkDir string
	BuildId string
}
