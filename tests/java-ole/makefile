default: compile test clean

compile:
	javac Main.java

test:
	../../bin/test
	@test "$$(wc -c < user.out | tr -d ' ')" -eq 1048576

clean:
	@rm Main.class user.err user.in user.out
