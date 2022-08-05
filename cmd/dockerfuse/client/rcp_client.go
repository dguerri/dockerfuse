package client

import (
	"io"
	"net/rpc"
)

var rpcCF rpcClientFactoryInterface = &rpcClientFactory{}

type rpcClientFactoryInterface interface {
	NewClient(conn io.ReadWriteCloser) rpcClient
}

type rpcClient interface {
	Call(serviceMethod string, args any, reply any) error
	Close() error
}

// rpcCL implements RPC client factory
type rpcClientFactory struct{}

func (*rpcClientFactory) NewClient(conn io.ReadWriteCloser) rpcClient { return rpc.NewClient(conn) }
