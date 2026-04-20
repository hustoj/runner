# HUSTOJ Runner

This project is judger runner for [HUSTOJ](https://github.com/hustoj/runner), written in golang.

## Platform Support

| Capability | Linux amd64 | Linux arm64 | macOS (dev) |
|---|---|---|---|
| Build (`make`) | ✅ | ✅ | ✅ |
| Unit tests (`go test ./...`) | ✅ | ✅ | ✅ |
| Run judge (`bin/runner`) | ✅ | ✅ | ❌ |
| Integration tests (`make testall`) | ✅ | ✅ | ❌ |

- **Runtime execution requires Linux (amd64 or arm64)**. The ptrace-based tracer and syscall tables are only available on these platforms.
- macOS is supported for **development, compilation, and type-checking only**. Darwin stubs exist solely to enable cross-platform IDE workflows.
- Other Linux architectures may compile successfully, but the runner will fail at startup because the syscall table is unavailable.

## Install

1. Install Golang 1.25 or newer (**MUST SUPPORT GOMODULES**)
2. clone this repo:

    ```sh
    git clone https://github.com/hustoj/runner.git
    ```
3. check environment

    ```sh
    cd runner
    make # will install go dependency
    make testall # will check exception detect is ok, should all passed
    ```

### `make testall` prerequisites

`make testall` compiles and runs the integration test cases under `tests/`. The following tools must be available:

- **C/C++ toolchain**: `gcc`, `g++` with static linking support (`libc-dev`, `libstdc++-dev`)
- **Java** (optional): `javac` and `java` for Java test cases (`tests/java*`)
- **make**: GNU Make

On Debian/Ubuntu:

```sh
sudo apt-get install build-essential default-jdk
```

4. install and enable pre-commit hooks

    ```sh
    pipx install pre-commit
    make pre-commit-install
    ```

    run all checks manually before commit:

    ```sh
    make pre-commit-run
    ```

    the same checks also run automatically in GitHub Actions for pushes to `main` / `master` and for pull requests.

5. Install docker

    if you instal debian series
    `https://docs.docker.com/install/linux/docker-ce/ubuntu/`

    or centos
    `https://docs.docker.com/install/linux/docker-ce/centos/`

6. build docker image

    ```bash
    make build-docker-compiler
    make build-docker-runner
    ```

    docker will build images
    the default version is v1 now.
