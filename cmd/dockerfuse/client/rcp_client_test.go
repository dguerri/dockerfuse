package client

import (
	"net"
	"net/rpc"
	"testing"
)

type echo int

func (e *echo) Echo(in string, out *string) error {
	*out = in
	return nil
}

func TestRPCClientFactoryNewClient(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	srv := rpc.NewServer()
	err := srv.RegisterName("Echo", new(echo))
	if err != nil {
		t.Fatalf("error registering server: %v", err)
	}
	go srv.ServeConn(serverConn)

	c := (&rpcClientFactory{}).NewClient(clientConn)
	defer c.Close()

	var reply string
	if err := c.Call("Echo.Echo", "hi", &reply); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if reply != "hi" {
		t.Fatalf("unexpected reply %s", reply)
	}
}
