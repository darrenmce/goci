package main

import (
	dkrCli "github.com/docker/docker/client"
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"strings"
	"bufio"
	log "github.com/sirupsen/logrus"
	"path"
)

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

func createDockerClient() (*dkrCli.Client, error) {
	return dkrCli.NewClientWithOpts(dkrCli.WithVersion("1.36"))
}

func runContainer(cli *dkrCli.Client, buildContainer BuildContainer, command string) (int, error) {
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
	check(err)

	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	check(err)

	reader, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	})
	check(err)
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
	check(err)


	err = cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})
	check(err)

	return status.State.ExitCode, nil
}
