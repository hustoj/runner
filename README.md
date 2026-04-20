# HUSTOJ Runner

This project is judger runner for [HUSTOJ](https://github.com/hustoj/runner), written in golang.

## Platform Support

| Capability | Linux amd64 | Linux arm64 | macOS (dev) |
|---|---|---|---|
| Build (`make`) | ✅ | ✅ | ✅ |
| Unit tests (`go test ./...`) | ✅ | ✅ | ✅ |
| Run judge (`bin/runner`) | ✅ | ✅ | ❌ |
| Integration tests (`make testall`) | ✅ | validate on target host | ❌ |

- **Runtime execution is implemented for Linux (amd64 and arm64)**. The ptrace-based tracer and syscall tables are only available on these platforms.
- macOS is supported for **development tasks, unit tests, compilation, and type-checking only**. Darwin stubs exist solely to enable cross-platform IDE workflows.
- Full integration coverage (`make testall`) is maintained on Linux/amd64. On Linux/arm64, run the suite on the target host before treating it as a release gate.
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
    make testall # Linux/amd64 integration check; run on your arm64 target host if needed
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
