Version := $(shell git describe --tags --dirty)
GitCommit := $(shell git rev-parse HEAD)
SATELLITE_LDFLAGS := "-extldflags '-static' -s -w -X main.Version=$(Version) -X main.GitCommit=$(GitCommit)"
DOCKERFUSE_LDFLAGS := "-s -w -X main.Version=$(Version) -X main.GitCommit=$(GitCommit)"

.PHONY: test quality_test all dockerfuse_satellite clean interactive_test interactive_test_fskit submodule

all: dockerfuse_satellite dockerfuse

submodule:
	true

dockerfuse_satellite_amd64: submodule cmd/satellite/main.go cmd/satellite/server/server.go pkg/rpccommon/rpc_types.go pkg/rpccommon/utils.go
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a \
		--ldflags $(SATELLITE_LDFLAGS) \
		-o dockerfuse_satellite_amd64 ./cmd/satellite/main.go

dockerfuse_satellite_arm64: submodule cmd/satellite/main.go cmd/satellite/server/server.go pkg/rpccommon/rpc_types.go pkg/rpccommon/utils.go
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a \
		--ldflags $(SATELLITE_LDFLAGS) \
		-o dockerfuse_satellite_arm64 ./cmd/satellite/main.go

dockerfuse_satellite: dockerfuse_satellite_amd64 dockerfuse_satellite_arm64

dockerfuse: submodule cmd/dockerfuse/main.go cmd/dockerfuse/backend_darwin.go cmd/dockerfuse/backend_other.go cmd/dockerfuse/client/client.go cmd/dockerfuse/client/dockerfuse_fs.go pkg/rpccommon/rpc_types.go pkg/rpccommon/utils.go
	env CGO_ENABLED=0 go build -a \
		--ldflags $(DOCKERFUSE_LDFLAGS) \
		-o dockerfuse ./cmd/dockerfuse

clean:
	rm -f dockerfuse_satellite_amd64 dockerfuse_satellite_arm64 dockerfuse

test: submodule
	go test ./...

quality_test: submodule
	go vet ./...
	go test ./... -cover
	golangci-lint run ./...
	gocyclo -top 10  -avg .

interactive_test: all
	docker kill dockerfuse-test || true
	docker run -d --rm --name dockerfuse-test alpine:latest sleep inf
	umount tmp || true
	./dockerfuse --debug -i dockerfuse-test -m ./tmp
	umount tmp || true
	docker kill dockerfuse-test || true

interactive_test_fskit: all
	docker kill dockerfuse-test || true
	docker run -d --rm --name dockerfuse-test alpine:latest sleep inf
	umount tmp || true
	@./dockerfuse --debug -i dockerfuse-test --backend=fskit -m ./tmp || (echo "\n*** ERROR ***\nPlease check if your OS supports FSKit (macOS 15.4+) and if MacFuse 5.2.0+ is installed."; exit 1)
	umount tmp || true
	docker kill dockerfuse-test || true
