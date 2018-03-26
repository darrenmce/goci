package main

type JobRun struct {
	InstructionFile string
	BuildId         string
	WorkDir         string
	Job             Job
}

type Job struct {
	Name string
	Git struct {
		Repo string
	}
	Build struct {
		Image string
		Steps []string
	}
	Publish struct {
		Repo     string
		Registry string
		authRef  string
	}
}

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
