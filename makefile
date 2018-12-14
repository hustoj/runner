MAKEFLAGS += -s

default:
	go build -o bin/runner ./cmd/runner

compiler:
	go build -o bin/compile ./cmd/compile

build-docker:
	go build -o docker/runner ./cmd/runner
	docker build -t runner:v1 ./docker

prepare:
	go build -o bin/test ./cmd/test

testall: prepare
	cd tests/general;make
	cd tests/fork;make
	cd tests/mle;make
	cd tests/mle2;make
	cd tests/mle21;make
	cd tests/segmentfault;make
	cd tests/socket;make
	cd tests/stack;make
	cd tests/thread;make
	cd tests/tle;make
	cd tests/tle2;make
	cd tests/zero;make