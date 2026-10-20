# Harnez's shared container-test target. Projects may set these variables in
# their Makefile to match their Containerfile and CI checks.
.PHONY: ⚙️ 🤖

container-test: ⚙️  # run project checks in an isolated container
	@if test "$${HARNEZ_QUOTA_1:-}" = 1 || { test -r /proc/self/ns/user && test "$$(readlink /proc/self/ns/user)" != "$$(readlink /proc/1/ns/user)"; }; then echo "container tests need the host; ask the host" >&2; exit 1; fi
	@if test -z "$(or $(CONTAINER_ENGINE),$(shell command -v podman || command -v docker))"; then echo "container-test requires podman or docker" >&2; exit 1; fi
	mkdir -p "$(or $(CONTAINER_OUTPUT),$(CURDIR)/.container-test-output)"
	env -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u NO_PROXY -u http_proxy -u https_proxy -u all_proxy -u no_proxy "$(or $(CONTAINER_ENGINE),$(shell command -v podman || command -v docker))" build --file "$(or $(CONTAINER_FILE),Containerfile)" --tag "$(or $(CONTAINER_IMAGE),container-test:local)" .
	env -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u NO_PROXY -u http_proxy -u https_proxy -u all_proxy -u no_proxy "$(or $(CONTAINER_ENGINE),$(shell command -v podman || command -v docker))" run --rm --volume "$(or $(CONTAINER_OUTPUT),$(CURDIR)/.container-test-output):/out:rw$(CONTAINER_OUTPUT_LABEL)" $(if $(filter mount,$(CONTAINER_SOURCE_MODE)),--volume "$(CURDIR):/src:ro$(CONTAINER_SOURCE_LABEL)",) --workdir /src "$(or $(CONTAINER_IMAGE),container-test:local)" sh -c '$(or $(CONTAINER_TEST_CMD),make test)'
