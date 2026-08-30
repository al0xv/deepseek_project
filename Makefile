BIN := bin

.PHONY: build test vet build-windows build-macos clean

build:
	mkdir -p $(BIN)
	go build -o $(BIN)/ds ./cmd/ds
	go build -o $(BIN)/dsgateway ./cmd/dsgateway

build-windows:
	mkdir -p $(BIN)
	GOOS=windows GOARCH=amd64 go build -o $(BIN)/ds.exe ./cmd/ds
	GOOS=windows GOARCH=amd64 go build -o $(BIN)/dsgateway.exe ./cmd/dsgateway

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf $(BIN)
