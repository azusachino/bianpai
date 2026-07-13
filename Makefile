.PHONY: build test test-loop smoke smoke-real vet fmt race coverage shellcheck quality check validate bizflow

build:
	go build ./...

test:
	go test ./...

# test-loop reruns the suite to shake out flakes: make test-loop COUNT=20
test-loop:
	scripts/test.sh $(COUNT)

# smoke drives every user-facing subcommand and all usecases with a fake
# `container` CLI.
smoke: build
	scripts/smoke.sh

# smoke-real drives each usecase with the real Apple `container` CLI and checks
# the exposed application endpoints.
smoke-real: build
	SMOKE_MODE=real scripts/smoke.sh

vet:
	go vet ./...

# race runs the suite with Go's race detector.
race:
	go test -race ./...

# coverage reports statement coverage by package without enforcing a threshold.
coverage:
	go test -cover ./...

# shellcheck validates the repository-owned shell scripts used by the checks.
shellcheck:
	shellcheck scripts/*.sh

# quality runs the additional checks that are cheap and deterministic in CI.
quality: race shellcheck

# fmt fails if any file is not gofmt-clean.
fmt:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# check is the pre-commit gate.
check: build test vet fmt

# validate is the pre-PR gate.
validate: check
	go mod verify

# bizflow runs a full business flow operation sequence on Apple container backend.
bizflow:
	scripts/bizflow.sh
