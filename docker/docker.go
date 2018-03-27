package docker

import (
	docker "github.com/docker/docker/client"
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"strings"
	"bufio"
	log "github.com/sirupsen/logrus"
	"path"
)

type ILib interface {
	RunContainer (bc BuildContainer, buildId string) (exitCode int, err error)
}

type Lib struct {
	Client *docker.Client
}

func CreateMounts(volumes []BuildVolume) []mount.Mount {
	var mounts []mount.Mount
	for _, vol := range volumes {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: vol.Source,
			Target: vol.Target,
		})
	}
	return mounts
}

func NewDockerLib() (ILib, error) {
	lib := Lib{}
	cli, err := docker.NewClientWithOpts(docker.WithVersion("1.36"))
	if err != nil {
		return lib, err
	}
	lib.Client = cli
	return lib, nil
}

func (dkr Lib) RunContainer(buildContainer BuildContainer, command string) (int, error) {
	cli := dkr.Client
	ctx := context.Background()

	log.WithFields(log.Fields{
		"Image":   buildContainer.Image,
		"Command": command,
	}).Info("Running Container")

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:        buildContainer.Image,
		Cmd:          strings.Split(command, " "),
		WorkingDir:   buildContainer.WorkDir,
		Tty:          true,
		AttachStderr: true,
		AttachStdout: true,
	}, &container.HostConfig{
		Mounts: CreateMounts(buildContainer.Volumes),
	}, nil, buildContainer.Name + "_" + path.Base(buildContainer.BuildId))

	if err != nil {
		return 0, err
	}

	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return 0, err
	}

	reader, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	})
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		log.Info(scanner.Text())
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)

	select {
	case runError := <-errCh:
		if runError != nil {
			return -1, runError
		}
	case <-statusCh:
	}

	status, err := cli.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return 0, err
	}


	err = cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})
	if err != nil {
		return 0, err
	}

	return status.State.ExitCode, nil
}
