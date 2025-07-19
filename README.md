# DockerFuse: interact with filesystem of Linux Docker containers, via FUSE

[![license](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0) [![go report card](https://goreportcard.com/badge/github.com/dguerri/dockerfuse)](https://goreportcard.com/report/github.com/dguerri/dockerfuse) [![CI](https://github.com/dguerri/dockerfuse/actions/workflows/run-CI.yml/badge.svg)](https://github.com/dguerri/dockerfuse/actions/workflows/run-CI.yml) [![Coverage Status](https://coveralls.io/repos/github/dguerri/dockerfuse/badge.svg?branch=main)](https://coveralls.io/github/dguerri/dockerfuse?branch=main)

DockerFuse lets you mount the filesystem of Linux Docker containers locally without installing additional services on the container.

![dockerfuse demo](doc/dockerfuse.gif)

## Build

DockerFuse is built using the provided Makefile. Running `make all` compiles the main `dockerfuse` binary and the architecture specific satellites used inside the container:

```bash
make all
```

The resulting files are `dockerfuse`, `dockerfuse_satellite_amd64` and `dockerfuse_satellite_arm64`.

## Running

Mount the root filesystem of a running container with:

```bash
sudo ./dockerfuse -i <container id or name> -m <mount point>
```

Specify `-path` to mount a sub directory and `-daemonize` to keep the process in the background.
DockerFuse can connect to remote Docker engines using the standard `DOCKER_HOST` environment variables.

## Makefile targets

- `make test` – run unit tests.
- `make quality_test` – run go vet, unit tests with coverage, golint and gocyclo.
- `make interactive_test` – pull the alpine image and mount it under `./tmp` for a quick demo.

## Testing

To run the unit tests:

```bash
make test
```

To run an interactive test that spawns an `alpine` container and mounts it under `./tmp`:

```bash
make interactive_test
```

To invoke Dockerfuse manually:

```bash
cd <dockerfuse dir>
make all
./dockerfuse -id <container id/name> -mount <mountpoint>
```

## FAQ

### Q. How does it work?

Dockerfuse uploads a small "server" on the container (the dockerfs satellite).
The satellite and the client (`dockerfuse`) communicate over stdin and stdout via the hijacked connection Docker Engine provides through `ContainerExecAttach()`.
This means no additional ports (or software, like ssh) is needed to remotely mount the docker filesystem.

Dockerfuse implements operations needed by FUSE through RPC calls and a satellite app. Dockerfuse satellite uses native systemcall (through the Go standard library) on the running container image. For filesystem operations, this is both faster and more flexible than using Docker Engine's API.

The obvious caveat is that Dockerfuse has to upload a small binary (i.e. ~ 4 MBytes) to the container.
The satellite is light-weight also for the computational power requirement, so it shouldn't affect your workload. Of course the actual load depends on the filesystem operations performed (and it should be noted that MacOS issues a huge number of `Getattr()` (STATFS) calls, potentially [affecting FUSE performances](https://github.com/hanwen/go-fuse#macos-support)).

### How is it different from other similar software?

Let's take two popular alternative implementations, Plesk's [plesk/docker-fs](https://github.com/plesk/docker-fs) and Microsoft's [Docker VSCode extension](https://marketplace.visualstudio.com/items?itemName=ms-azuretools.vscode-docker) filesystem browsing.

Plesk's Docker-fs uses the Docker Engine to provide access to the container's filesystem, and that has important limitations.
To name some: Docker-fs has to download the whole container image on start (using it as a tar FS with FUSE) and use it as a tar-based FS with FUSE (making initial access very slow). It cannot create empty directories, and it cannot handle large files (read/writes operates on the entire file).

MS's Docker VSCode extension has many features, and it's perfectly integrated in VSCode. For filesystem browsing, it uses [Microsoft vscode-container-client](https://www.npmjs.com/package/@microsoft/vscode-container-client), which is a reusable Node.js package. It also supports both Linux and Windows containers.

When it comes to Linux fs, Microsoft's vscode-container-client npm issues shell commands to the running Docker container and parses the output (e.g., [list files](https://github.com/microsoft/vscode-docker-extensibility/blob/ac9703e17c143eedc069e3daba64e758b3326fd8/packages/vscode-container-client/src/clients/DockerClientBase/DockerClientBase.ts#L1609)). The cons of this approach are obviously that the container needs to have those commands and their output should be understood by microsoft/vscode-container-client.

In short, microsoft/vscode-container-client won't work on distroless containers or containers that don't include a shell (and the commands output parsing can be fragile).

### Q. Does it work on remote Docker servers?

Yeah. Dockerfuse can work on local Docker instances or on remote ones.
It uses the environment (i.e., `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`) to connect to the Docker server, and then it operates via TCP(*).

(*) Technically, Dockerfuse uses a TCP connection which is "upgraded" from an HTTP connection, similarly to what happens with web sockets.

### Q. Does it work for arm64 containers?

Yeah. The makefile creates 2 satellite instances, one for amd64 and one for arm64.
When mounting a remote container, Dockerfuse inspect the related image and uploads the right satellite instance.

This allows you to mount the filesystem of an arm64 container on an amd64 machine, and the way around.

### Q. Does it work on distroless containers?

Yup! Matter of fact Dockerfuse works great on minimal Docker containers, even when there is no shell installed.

### Q. Does it work on Windows containers?

Nope. Although it shouldn't be to hard to code, there is no support for Windows containers at this time.

### Q. Does it require root privileges?

Yes. Dockerfuse mounts the filesystem directly using FUSE and therefore needs privileges to perform the mount operation. Run it with `sudo` or as a user allowed to mount FUSE filesystems.

### Q. Can I mount only a sub directory of a container?

Absolutely. Use the `-path` option to specify the directory inside the container that you want to mount.

## License

Apache License v2. See LICENSE.TXT for details.

## Author

Davide Guerri <davide.guerri@gmail.com>
