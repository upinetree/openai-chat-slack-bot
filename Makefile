.PHONY: build package clean

BINARY_NAME := main
PACKAGE_NAME := deployment.zip

build:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME) main.go

package: build
	zip $(PACKAGE_NAME) $(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME) $(PACKAGE_NAME)
