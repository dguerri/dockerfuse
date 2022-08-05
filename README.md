# DockerFuse: interact with Docker conainters filesystem via FUSE

***NOTE: this software is a WIP, use at your risk!***

DockerFuse allows mounting the filesystem of Docker containers locally.

## Testing

To run Unit tests (very few for now):

```bash
make test
```

To run an interactive test, pulling `alpine` image, spinning up a container and mounting its filesystem on ./tmp:

```bash
make interactive_test
```

## FAQ

### Q. How does it work?

Dockerfuse uploads a small "server" on the container (i.e. the dockerfs satellite).
The satellite and the client (`dockerfuse`) communicate over stdin and stdout via the hijacked connection Docker Engine provides through `ContainerExecAttach()`
This means no additional ports (or software, like ssh) is needed to remotely mount the docker filesystem.

### Q. How is it different from [plesk/docker-fs](https://github.com/plesk/docker-fs)?

`plesk/docker-fs` uses Docker Engine to provide access to the container's filesystem, and that has big limitations.
`dockerfuse` implements operations needed by FUSE through RPC calls and a small (i.e. < 5 MB) satellite. For filesystem operations, this is both faster and more flexible than using Docker Engine's API.

### Q. Does it work on remote Docker servers?

Yeah. Dockerfuse can work on local Docker instances or on remote ones.
It uses the environment (i.e., DOCKER_HOST, DOCKER_TLS_VERIFY, DOCKER_CERT_PATH) to connect to the Docker server, and then it operates via TCP(*).

(*) Tecnically Dockerfuse uses a TCP connection which is "upgraded" from an HTTP connection, similarly to what happens with web sockets.

### Q. Does it work for arm64 containers?

Yeah. The makefile creates 2 satellite instances, one for amd64 and one for arm64.
When mountig a remote container, Dockerfuse inspect the related image and uploads the right satellite instance.

## License

Apache License v2. See LICENSE.TXT for details.

## Author

Davide Guerri <davide.guerri@gmail.com>
