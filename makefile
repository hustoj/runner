MAKEFLAGS += -s

default:
	go build -o bin/runner ./cmd/runner

compiler:
	go build -o bin/compile ./cmd/compiler

build-docker-runner:
	go build -o docker/runner/runner ./cmd/runner
	docker build -t runner:v1 ./docker/runner

build-docker-compiler:
	go build -o docker/compiler/compiler ./cmd/compiler
	docker build -t compiler:v1 ./docker/compiler

prepare:
	go build -o bin/test ./cmd/test

testall: prepare
	cd tests/general;make
	cd tests/fork;make
	cd tests/mle;make
	cd tests/mle2;make
	cd tests/mle21;make
	cd tests/mle3;make
	cd tests/segmentfault;make
	cd tests/socket;make
	cd tests/stack;make
	cd tests/thread;make
	cd tests/tle;make
	cd tests/tle2;make
	cd tests/zero;make
	cd tests/ole;make
	cd tests/java;make
	cd tests/java-tle;make
	cd tests/java-mle;make

clean:
	@rm bin/runner
	@rm bin/test