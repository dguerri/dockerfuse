package client

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

var dockerCF dockerClientFactoryInterface = &dockerClientFactory{}

type dockerClientFactoryInterface interface {
	NewClientWithOpts(ops ...client.Opt) (dockerClient, error)
}

type dockerClient interface {
	ContainerExecAttach(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error)
	ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	CopyToContainer(ctx context.Context, containerID, dstPath string, content io.Reader, options container.CopyToContainerOptions) error
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
}

// dockerClientFactory implements dockerClientFactoryInterface providing real client for Docker API
type dockerClientFactory struct{}

func (*dockerClientFactory) NewClientWithOpts(ops ...client.Opt) (dockerClient, error) {
	return client.NewClientWithOpts(ops...)
}
