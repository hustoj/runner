default:
	go build -o bin/runner cmd/runner.go

test:
	go build -o bin/test cmd/test.go

	gcc ./tests/main.c -o bin/main -static
	@rm bin/Main;ln -s main bin/Main
	@cd bin;./test 1 2 4

	gcc ./tests/mle.c -o bin/mle -static
	@rm bin/Main;ln -s mle bin/Main
	@cd bin;./test 10 4 8

	gcc ./tests/mle2.c -o bin/mle2 -static
	@rm bin/Main;ln -s mle2 bin/Main
	@cd bin;./test 10 4 8

	gcc ./tests/mle3.c -o bin/mle3 -static
	@rm bin/Main;ln -s mle3 bin/Main
	@cd bin;./test 20 5 8

	gcc ./tests/tle.c -o bin/tle -static
	@rm bin/Main;ln -s tle bin/Main
	@cd bin;./test 1 1 8

	gcc ./tests/tle2.c -o bin/tle2 -static
	@rm bin/Main;ln -s tle2 bin/Main
	@cd bin;./test 1 1 8