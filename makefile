default:
	go build -o bin/runner cmd/runner.go

build-docker:
	go build -o docker/runner cmd/runner.go
	docker build -t runner:v1 ./docker

test: prepare case-general case-mle1 case-mle2 \
	case-mle3 case-tle case-tle2 case-fork case-thread

prepare:
	go build -o bin/test cmd/test.go

case-general:
	gcc ./tests/main.c -o bin/general -static
	@rm bin/main;ln -s general bin/main
	@cd bin;./test -cpu=1 -memory=2 -result=4

case-mle1:
	gcc ./tests/mle.c -o bin/mle -static
	@rm bin/main;ln -s mle bin/main
	@cd bin;./test -cpu=10 -memory=4 -result=8

case-mle2:
	gcc ./tests/mle2.c -o bin/mle2 -static
	@rm bin/main;ln -s mle2 bin/main
	@cd bin;./test -cpu=10 -memory=4 -result=8

case-mle3:
	gcc ./tests/mle3.c -o bin/mle3 -static
	@rm bin/main;ln -s mle3 bin/main
	@cd bin;./test -cpu=20 -memory=4 -result=8

case-tle:
	gcc ./tests/tle.c -o bin/tle -static
	@rm bin/main;ln -s tle bin/main
	@cd bin;./test -cpu=1 -memory=2 -result=7

case-tle2:
	gcc ./tests/tle2.c -o bin/tle2 -static
	@rm bin/main;ln -s tle2 bin/main
	@cd bin;./test -cpu=1 -memory=2 -result=7

case-fork:
	gcc ./tests/fork.c -o bin/fork -static
	@rm bin/main;ln -s fork bin/main
	@cd bin;./test -cpu=10 -memory=10 -result=10

case-thread:
	g++ tests/thread.cpp -o bin/thread -static -std=gnu++11 -lpthread
	@rm bin/main;ln -s thread bin/main
	@cd bin;./test -cpu=10 -memory=10 -result=10