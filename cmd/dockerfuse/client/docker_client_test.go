package client

import (
	"testing"

	"github.com/docker/docker/client"
)

func TestDockerClientFactoryNewClientWithOpts(t *testing.T) {
	c, err := (&dockerClientFactory{}).NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("nil client returned")
	}
	// ensure returned type is *client.Client which implements dockerClient
	if _, ok := c.(*client.Client); !ok {
		t.Fatalf("expected *client.Client, got %T", c)
	}
}
