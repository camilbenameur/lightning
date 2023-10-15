package runner

import (
	"bytes"
	"context"
	util "lightning/internal/utils"
	"log"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type Runner struct {
	docker *client.Client
}

const (
	RUNNERS_PATH = "/tmp/runners/"
)

func NewRunner() *Runner {

	os.Mkdir(RUNNERS_PATH, 0666)

	return &Runner{
		docker: NewDockerClient(),
	}
}

type RunPayload struct {
	Script   string `json:"script"`
	Language string `json:"language"`
}

func (runner *Runner) Run(payload RunPayload) (string, error) {

	id := util.NewId()

	os.Mkdir(RUNNERS_PATH+id, 0666)

	file, err := os.Create(RUNNERS_PATH + id + "/main.py")
	if err != nil {
		log.Println("Error creating file", err)
		return "", err
	}
	defer file.Close()

	_, err = file.WriteString(payload.Script)

	if err != nil {
		log.Println("Error writing file", err)
		return "", err
	}

	_, err = runner.docker.ContainerCreate(context.Background(), &container.Config{
		Image: "python",
		Cmd:   []string{"python3", "/app/main.py"},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: RUNNERS_PATH + id,
				Target: "/app",
			},
		},
	}, &network.NetworkingConfig{}, &v1.Platform{}, id)
	if err != nil {
		log.Println("Error creating container", err)
		return "", err
	}

	err = runner.docker.ContainerStart(context.Background(), id, types.ContainerStartOptions{})
	if err != nil {
		log.Println("Error starting container", err)
		return "", err
	}

	res, _ := runner.docker.ContainerWait(context.Background(), id, container.WaitConditionNextExit)
	<-res
	reader, err := runner.docker.ContainerLogs(context.Background(), id, types.ContainerLogsOptions{ShowStdout: true, Tail: "all"})

	if err != nil {
		log.Println("Error getting logs", err)
		return "", err
	}

	buf := bytes.NewBuffer([]byte{})
	buf.ReadFrom(reader)

	log.Println(buf.String())

	return buf.String(), nil
}
