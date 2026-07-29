BINARY := llamadeck

.PHONY: build install test e2e fmt vet clean run

build:
	go build -o $(BINARY) .

install:
	go install .

run: build
	./$(BINARY)

test: vet
	go test ./...

e2e:
	@test -n "$(MODEL)" || (echo "uso: make e2e MODEL=/caminho/modelo.gguf"; exit 1)
	LLAMADECK_E2E=1 LLAMADECK_E2E_MODEL=$(MODEL) go test ./internal/server -run RealLlama -v -timeout 180s

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
