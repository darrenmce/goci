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
	"bytes"
)

type ContainerRunner interface {
	RunContainer (bc BuildContainer, buildId string) (exitCode int, err error)
}

type ImagePublisher interface {
	PublishImage (i Image, ra RepoAuth, tag string) (err error)
}

func (dkr containerRunner) RunContainer(buildContainer BuildContainer, command string) (int, error) {
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
		Mounts: createMounts(buildContainer.Volumes),
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

type containerRunner struct {
	Client *docker.Client
}

func NewContainerRunner(version string) (ContainerRunner, error) {
	lib := containerRunner{}
	cli, err := docker.NewClientWithOpts(docker.WithVersion(version))
	if err != nil {
		return lib, err
	}
	lib.Client = cli
	return lib, nil
}

func createMounts(volumes []BuildVolume) []mount.Mount {
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

type imagePublisher struct {
	Client *docker.Client
}


type Image struct {
	TarContents []byte
	Name string
	Dockerfile string
}

func (image Image) NewImageFromPath(path string) (Image, error) {

}

type RepoAuth struct {

}

func (ip imagePublisher) PublishImage(image Image, repoAuth RepoAuth, tag string) (error) {
	cli := ip.Client
	ctx := context.Background()
	buildResponse, err := cli.ImageBuild(ctx, nil, types.ImageBuildOptions{
		Context: bytes.NewReader(image.TarContents),
		Dockerfile: image.Dockerfile,
		Target: image.Name,
		Tags: []string{ tag },
	})
	if err != nil {
		return err
	}
	defer buildResponse.Body.Close();
	return nil
}

func NewImagePublisher(version string) (ImagePublisher, error) {
	publisher := imagePublisher{}
	cli, err := docker.NewClientWithOpts(docker.WithVersion(version))
	if err != nil {
		return publisher, err
	}
	publisher.Client = cli
	return publisher, nil
}


