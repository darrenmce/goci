package main

import (
	log "github.com/sirupsen/logrus"
	"os"
	"io/ioutil"
	"gopkg.in/urfave/cli.v1"
	"gopkg.in/yaml.v2"
	"text/template"
	"github.com/darrenmce/goci/docker"
)

func check(e error) {
	if e != nil {
		log.WithError(e).Panic("Fatal Error!")
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "GoCI"
	app.Usage = "run some stuff in docker"
	app.Description = "A simple CI job runner"

	flags := []cli.Flag{
		cli.StringFlag{
			Name:  "buildId, b",
			Usage: "Build ID/Number",
		},
		cli.StringFlag{
			Name:  "instructions, i",
			Usage: "path to YAML instructions",
		},
	}

	app.Flags = flags

	app.Action = runner
	err := app.Run(os.Args)
	check(err)
}

func runner(c *cli.Context) {
	instructionFile := c.String("instructions")
	buildId := c.String("buildId")

	yamlData, err := ioutil.ReadFile(instructionFile)
	check(err)

	run := JobRun{BuildId: buildId}
	err = yaml.Unmarshal([]byte(yamlData), &run)
	check(err)

	job := run.Job
	run.WorkDir = getWorkDir(job.Name)

	tmpl, err := getJobRunTemplate()
	check(err)
	tmpl.Execute(log.StandardLogger().Writer(), run)

	err = checkout(job.Git.Repo, run.WorkDir, log.StandardLogger())
	check(err)

	dkr, err := docker.NewDockerLib()
	check(err)

	buildStatusCode, err := run.RunBuild(dkr, buildId)
	check(err)

	if buildStatusCode != 0 {
		log.Error("DOH!")
	}
}

func (run JobRun) RunBuild (dkr docker.ILib, buildId string) (int, error) {
	exitCode := 0
	job := run.Job

	buildContainer := docker.BuildContainer{
		Name: job.Name,
		Image: job.Build.Image,
		Volumes: []docker.BuildVolume{ {run.WorkDir,"/build"} },
		WorkDir: "/build",
		BuildId: buildId,
	}

	for _, step := range job.Build.Steps {
		statusCode, err := dkr.RunContainer(buildContainer, step)

		if err != nil {
			return statusCode, err
		}

		log.WithFields(log.Fields{
			"statusCode": statusCode,
		}).Info("CONTAINER exited")

		if statusCode != 0 {
			exitCode = statusCode
			break
		}
	}

	return exitCode, nil
}

func getJobRunTemplate() (*template.Template, error) {
	jobTemplate, err := template.New("JobTemplate").Parse(
		"Running Job (Build ID: {{.BuildId}})\n" +
			"Name: {{.Job.Name}}\n" +
			"Repo: {{.Job.Git.Repo}}\n" +
			"Working Directory: {{.WorkDir}}",
	)
	check(err)
	return jobTemplate, err
}

func getWorkDir(name string) (dir string) {
	dir, err := ioutil.TempDir("", name)
	check(err)
	return dir
}

