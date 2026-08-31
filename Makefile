BIN := bin

.PHONY: build test vet build-windows build-macos build-linux build-linux-amd64 build-linux-arm64 clean ios-plist-check

build:
	mkdir -p $(BIN)
	go build -o $(BIN)/ds ./cmd/ds
	go build -o $(BIN)/dsgateway ./cmd/dsgateway

build-windows:
	mkdir -p $(BIN)
	GOOS=windows GOARCH=amd64 go build -o $(BIN)/ds.exe ./cmd/ds
	GOOS=windows GOARCH=amd64 go build -o $(BIN)/dsgateway.exe ./cmd/dsgateway

# Static Linux gateway binaries for Oracle Cloud Always Free VMs (amd64 / arm64).
# No Go toolchain is required on the server.
build-linux-amd64:
	mkdir -p $(BIN)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BIN)/dsgateway-linux-amd64 ./cmd/dsgateway

build-linux-arm64:
	mkdir -p $(BIN)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(BIN)/dsgateway-linux-arm64 ./cmd/dsgateway

build-linux: build-linux-amd64 build-linux-arm64

test:
	go test ./...

vet:
	go vet ./...

# Verifies the iOS source Info.plist contains the required privacy usage
# descriptions (NSFaceIDUsageDescription among them) and no forbidden ones.
ios-plist-check:
	scripts/check-ios-plist.sh

clean:
	rm -rf $(BIN)
