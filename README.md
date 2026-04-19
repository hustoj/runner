# HUSTOJ Runner

This project is judger runner for [HUSTOJ](https://github.com/hustoj/runner), written in golang.

## Install

1. Install Golang 1.25 or newer (**MUST SUPPORT GOMODULES**)
   - This project uses Go 1.25 features and strict type checking.
   - It supports cross-platform development (Linux/macOS), but core tracing features are only functional on Linux.
2. clone this repo:

    ```sh
    git clone https://github.com/hustoj/runner.git
    ```
3. check enviroment

    ```sh
    cd runner
    make # will install go depencency
    make testall # will check exception detect is ok, should all passed
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
