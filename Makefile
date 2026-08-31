BIN := bin

.PHONY: build test vet build-windows build-macos clean ios-plist-check

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

# Verifies the iOS source Info.plist contains the required privacy usage
# descriptions (NSFaceIDUsageDescription among them) and no forbidden ones.
ios-plist-check:
	scripts/check-ios-plist.sh

clean:
	rm -rf $(BIN)
