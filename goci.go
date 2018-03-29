package main

import (
	log "github.com/sirupsen/logrus"
	"os"
	"io/ioutil"
	"gopkg.in/urfave/cli.v1"
	"gopkg.in/yaml.v2"
	"text/template"
	"github.com/darrenmce/goci/docker"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/spf13/viper"
)

func check(e error) {
	if e != nil {
		log.WithError(e).Panic("Fatal Error!")
	}
}

func initConfig() {
	viper.SetConfigName("goci")
	viper.SetConfigType("yaml")
	viper.SetConfigFile(".goci.yml")
	err := viper.ReadInConfig()
	check(err)
}

func main() {
	initConfig()
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

func runner(c *cli.Context) (error) {
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

	containerRunner, err := docker.NewContainerRunner("1.36")
	check(err)

	buildStatusCode, err := run.RunBuild(containerRunner, buildId)
	check(err)

	if buildStatusCode != 0 {
		log.Error("DOH!")
		return errors.New("build failed")
	}

	imagePublisher, err := docker.NewImagePublisher("1.36")
	check(err)

	err = run.RunPublish(imagePublisher)
	check(err)

	return nil
}

func (run JobRun) RunBuild (dkr docker.ContainerRunner, buildId string) (int, error) {
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

func NewAuthConfigFromViper(registry string, ref string) types.AuthConfig {
	return types.AuthConfig{
		ServerAddress: registry,
		Username: viper.GetString("registries."+ ref +".username"),
		Password: viper.GetString("registries."+ ref +".password"),
	}
}

func (run JobRun) RunPublish (publisher docker.ImagePublisher) (error) {
	pub := run.Job.Publish
	image, err := docker.NewImageFromDir("darrenmce/gocitest", run.WorkDir)
	if err != nil {
		return err
	}
	imageAndTag, err := publisher.BuildImage(image, "stable")
	if err != nil {
		return err
	}


	err = publisher.PublishImage(imageAndTag, NewAuthConfigFromViper(pub.Registry, pub.AuthRef))
	if err != nil {
		return err
	}
	return nil
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

