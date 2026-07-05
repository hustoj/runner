default: compile test assert clean

compile:
	gcc main.c -o main -static

test:
	@../../bin/test > test.log

assert:
	@grep -q "passed!" test.log

clean:
	@rm -f main user.err user.in user.out test.log
