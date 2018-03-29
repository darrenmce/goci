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
	"github.com/mholt/archiver"
	"os"
	"io/ioutil"
	"encoding/json"
	"encoding/base64"
)

type ContainerRunner interface {
	RunContainer (bc BuildContainer, buildId string) (exitCode int, err error)
}

type ImagePublisher interface {
	BuildImage (i Image, tag string) (imageNameAndTag string, err error)
	PublishImage (imageNameAndTag string, authConfig types.AuthConfig) (error)
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
	Ctx context.Context
}

func NewContainerRunner(version string) (ContainerRunner, error) {
	lib := containerRunner{}
	cli, err := docker.NewClientWithOpts(docker.WithVersion(version))
	if err != nil {
		return lib, err
	}
	lib.Client = cli
	lib.Ctx = context.Background()
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
	Ctx context.Context
}


type Image struct {
	BuildContextTar string
	Name            string
}

func NewImageFromDir(name string, dir string) (Image, error) {
	res := Image{
		Name: name,
	}

	tarPath := path.Join(dir, "context.tar")
	contents, err := ioutil.ReadDir(dir)
	if err != nil {
		return res, err
	}

	//create a []string of file paths
	files := make([]string, len(contents))
	for i, fileInfo := range contents {
		files[i] = path.Join(dir, fileInfo.Name())
	}

	//tar all the files in the directory
	err = archiver.Tar.Make(tarPath, files)
	if err != nil {
		return res, err
	}

	res.BuildContextTar = tarPath

	return res, nil
}

type Registry struct {
	Uri string
	AuthConfig types.AuthConfig
}

func (ip imagePublisher) BuildImage(image Image, tag string) (imageNameAndTag string, err error) {
	cli := ip.Client

	buildContextTarFile, err := os.Open(image.BuildContextTar)
	if err != nil {
		return "", err
	}
	defer buildContextTarFile.Close()

	imageAndTag := image.Name + ":" + tag
	buildResponse, err := cli.ImageBuild(ip.Ctx, buildContextTarFile, types.ImageBuildOptions{
		Tags: []string{imageAndTag},
	})
	if err != nil {
		return "", err
	}

	defer buildResponse.Body.Close()
	scanner := bufio.NewScanner(buildResponse.Body)

	for scanner.Scan() {
		log.Info(scanner.Text())
	}

	return imageAndTag, nil
}

func (ip imagePublisher) PublishImage(imageNameAndTag string, authConfig types.AuthConfig) (error) {
	cli := ip.Client

	encodedJSONAuth, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}

	auth64 := base64.URLEncoding.EncodeToString(encodedJSONAuth)

	response, err := cli.ImagePush(ip.Ctx, imageNameAndTag, types.ImagePushOptions{
		RegistryAuth: auth64,
	})
	if err != nil {
		return err
	}
	defer response.Close()
	scanner := bufio.NewScanner(response)

	for scanner.Scan() {
		log.Info(scanner.Text())
	}
	return nil
}


func NewImagePublisher(version string) (ImagePublisher, error) {
	publisher := imagePublisher{}
	cli, err := docker.NewClientWithOpts(docker.WithVersion(version))
	if err != nil {
		return publisher, err
	}
	publisher.Client = cli
	publisher.Ctx = context.Background()
	return publisher, nil
}


