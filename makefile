default:
	go build -o bin/runner cmd/runner.go

build-docker:
	go build -o docker/runner cmd/runner.go
	docker build -t runner:v1 ./docker

test: prepare case-general case-mle case-mle2 \
	case-mle3 case-tle case-tle2 case-fork case-thread

prepare:
	go build -o bin/test cmd/test.go

case-general:
	gcc ./tests/main.c -o bin/main -static
	@rm bin/Main;ln -s main bin/Main
	@cd bin;./test 1 2 4

case-mle1:
	gcc ./tests/mle.c -o bin/mle -static
	@rm bin/Main;ln -s mle bin/Main
	@cd bin;./test 10 4 8

case-mle2:
	gcc ./tests/mle2.c -o bin/mle2 -static
	@rm bin/Main;ln -s mle2 bin/Main
	@cd bin;./test 10 4 8

case-mle3:
	gcc ./tests/mle3.c -o bin/mle3 -static
	@rm bin/Main;ln -s mle3 bin/Main
	@cd bin;./test 20 5 8

case-tle:
	gcc ./tests/tle.c -o bin/tle -static
	@rm bin/Main;ln -s tle bin/Main
	@cd bin;./test 1 1 8

case-tle2:
	gcc ./tests/tle2.c -o bin/tle2 -static
	@rm bin/Main;ln -s tle2 bin/Main
	@cd bin;./test 1 1 8

case-fork:
	gcc ./tests/fork.c -o bin/fork -static
	@rm bin/Main;ln -s fork bin/Main
	@cd bin;./test 1 10 10

case-thread:
	g++ tests/thread.c -o bin/thread -static -std=gnu++11 -lpthread
	@rm bin/Main;ln -s thread bin/Main
	@cd bin;./test 1 10 10