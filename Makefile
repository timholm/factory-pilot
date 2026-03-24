.PHONY: build test clean run diagnose report docker deploy

BINARY := factory-pilot
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test ./... -v -count=1

clean:
	rm -f $(BINARY)

run: build
	./$(BINARY) run

diagnose: build
	./$(BINARY) diagnose

report: build
	./$(BINARY) report

serve: build
	./$(BINARY) serve

docker:
	docker build -t factory-pilot:$(VERSION) .

deploy:
	kubectl apply -f deploy/deployment.yaml

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
