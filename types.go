package main

import "github.com/darrenmce/goci/docker"

type IJobRun interface {
	RunBuild (lib docker.ILib, buildId string) (int, error)
}

type JobRun struct {
	InstructionFile string
	BuildId         string
	WorkDir         string
	Job             Job
}

type Job struct {
	Name string
	Git  JobGit
	Build JobBuild
	Publish JobPublish
}

type JobGit struct {
	Repo string
}

type JobBuild struct {
	Image string
	Steps []string
}
type JobPublish struct {
	Repo     string
	Registry string
	AuthRef  string
}
