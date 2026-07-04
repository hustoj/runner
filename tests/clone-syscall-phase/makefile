default: compile test assert clean

compile:
	@rm -f sentinel.dest
	@touch sentinel.source
	gcc main.c -o main -static

test:
	../../bin/test

assert:
	@test -e sentinel.source
	@test ! -e sentinel.dest

clean:
	@rm -f main user.err user.in user.out sentinel.source sentinel.dest
