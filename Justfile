build:
    @echo "Building..."
    @go build -o $(BINARY_NAME) -v

test:
    @echo "Testing..."
    @go test -v

copy-driver-code:
    #!/bin/bash
    mkdir tmpcpy
    find mysql -name '*.go' ! -name '*_test.go' -exec cp {} tmpcpy \;
    cd tmpcpy
    for f in *.go; do
        cat $f >tmp && echo '// THIS FILE IS SYNCED FROM THE MYSQL DRIVER UPSTREAM, DO NOT MODIFY' >$f && cat tmp >>$f && mv tmp f
    done
    cd ..
    mv tmpcpy/* .
    rm -r tmpcpy
    sed -i '' -e 's/package mysql$/package mysqlauthproxy/' *.go
