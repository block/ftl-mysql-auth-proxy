build:
    @echo "Building..."
    @go build -o $(BINARY_NAME) -v

test:
    @echo "Testing..."
    @go test -v