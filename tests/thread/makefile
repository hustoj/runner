default: compile test clean

compile:
	g++ thread.cpp -o main -static -std=gnu++11 -lpthread

test:
	../../bin/test

clean:
	@rm -f main user.err user.in user.out
