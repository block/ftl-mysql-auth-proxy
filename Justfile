build:
    @echo "Building..."
    @go build -o $(BINARY_NAME) -v

test:
    @echo "Testing..."
    @go test -v

copy-driver-code:
    @echo "Copying driver code..."
    find mysql -name '*.go' ! -name '*_test.go' -exec cp {} . \;
    sed -i '' -e 's/package mysql$/package mysqlauthproxy/' *.go
