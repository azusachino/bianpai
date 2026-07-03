.PHONY: build test vet fmt check validate

build:
	go build ./...

test:
	go test ./...

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
