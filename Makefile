.PHONY: build test test-loop smoke smoke-real vet fmt check validate

build:
	go build ./...

test:
	go test ./...

# test-loop reruns the suite to shake out flakes: make test-loop COUNT=20
test-loop:
	scripts/test.sh $(COUNT)

# smoke drives up/down/ps against every usecase with a fake `container` CLI.
smoke: build
	scripts/smoke.sh

# smoke-real drives each usecase with the real Apple `container` CLI and checks
# the exposed application endpoints.
smoke-real: build
	SMOKE_MODE=real scripts/smoke.sh

vet:
	go vet ./...

# fmt fails if any file is not gofmt-clean.
fmt:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# check is the pre-commit gate.
check: build test vet fmt

# validate is the pre-PR gate.
validate: check
	go mod verify
