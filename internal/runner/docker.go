package runner

import (
	"log"

	"github.com/docker/docker/client"
)

func NewDockerClient() *client.Client {
	client, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		log.Fatal(err)
	}

	return client
}
