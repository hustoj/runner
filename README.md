# HUSTOJ Runner

This project is judger runner for [HUSTOJ](https://github.com/hustoj/runner), written in golang.

## Install 

1. Install Golang 1.13 or newer (**MUST SUPPORT GOMODULES**)
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

4. Install docker

    if you instal debian series
    `https://docs.docker.com/install/linux/docker-ce/ubuntu/`
    
    or centos 
    `https://docs.docker.com/install/linux/docker-ce/centos/`

5. build docker image

    ```bash
    make build-docker-compiler
    make build-docker-runner
    ```
    
    docker will build images
    the default version is v1 now.
