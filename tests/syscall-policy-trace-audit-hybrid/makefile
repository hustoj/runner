default: compile test assert clean

compile:
	@rm -f runner.log test.log
	gcc main.c -o main -static

test:
	@../../bin/test > test.log

assert:
	@grep -q "passed!" test.log
	@grep -q "audit syscall source=seccomp.*getppid" runner.log

clean:
	@rm -f main user.err user.in user.out runner.log test.log
