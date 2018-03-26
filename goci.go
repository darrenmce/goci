package main

import (
	log "github.com/sirupsen/logrus"
	"os"
	"io/ioutil"
	"gopkg.in/urfave/cli.v1"
	"gopkg.in/yaml.v2"
	"text/template"
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
	failed := false

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

	docker, err := createDockerClient()
	check(err)

	buildContainer := BuildContainer{
		Name: job.Name,
		Image: job.Build.Image,
		Volumes: []BuildVolume{ {run.WorkDir,"/build"} },
		WorkDir: "/build",
		BuildId: buildId,
	}

	for _, step := range job.Build.Steps {
		status, err := runContainer(docker, buildContainer, step)
		check(err)

		log.WithFields(log.Fields{
			"status": status,
		}).Info("CONTAINER exited")

		if status != 0 {
			failed = true
			break
		}
	}

	if failed {
		log.Error("DOH!")
	}
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

