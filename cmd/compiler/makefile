default:
	go build .

test-c:
	@cd ./tests/c/;../../compiler

test-cpp:
	@cd ./tests/cpp;../../compiler

test-fpc:
	@cd ./tests/fpc;../../compiler

test-16g:
	@cd ./tests/16g;../../compiler

test-bigexe:
	@cd ./tests/bigexe;../../compiler

test-cgroup-cleanup:
	@cd ./tests/cgroup-cleanup; \
		rm -f compile.out compile.err result.json; \
		run_uid=$${COMPILECONFIG_RUNUID:-$${SUDO_UID:-$$(id -u)}}; \
		run_gid=$${COMPILECONFIG_RUNGID:-$${SUDO_GID:-$$(id -g)}}; \
		if [ "$$(id -u)" -eq 0 ] && { [ "$$run_uid" -le 0 ] || [ "$$run_gid" -le 0 ]; }; then \
			echo "root compiler cleanup test requires COMPILECONFIG_RUNUID/RUNGID or sudo SUDO_UID/SUDO_GID"; \
			exit 1; \
		fi; \
		COMPILECONFIG_RUNUID=$$run_uid COMPILECONFIG_RUNGID=$$run_gid ../../compiler > result.json; \
		grep -q '"success":true' result.json; \
		sleep 0.5; \
		if pgrep -x sleep -a | grep ' 7784$$'; then \
			echo "residual compiler helper process found"; \
			exit 1; \
		fi

test-bomb:
	@cd ./tests/16g;../../compiler
