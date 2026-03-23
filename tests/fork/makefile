default: compile test clean

compile:
	gcc fork.c -o main -static

test:
	../../bin/test

clean:
	@rm -f main user.err user.in user.out
