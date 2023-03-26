.PHONY: build package clean

BINARY_NAME := main
PACKAGE_NAME := deployment.zip
LAMBDA_FUNC_NAME := MyGolangOpenAIFunction

build:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME) main.go

package: build
	zip $(PACKAGE_NAME) $(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME) $(PACKAGE_NAME)

# Requires env LAMBDA_ROLE_ARN
provision:
	aws lambda create-function \
		--function-name $(LAMBDA_FUNC_NAME) \
		--runtime go1.x \
		--role $(LAMBDA_ROLE_ARN) \
		--handler $(BINARY_NAME) \
		--zip-file fileb://$(PACKAGE_NAME) \
		--region ap-northeast-1
	aws lambda create-function-url-config \
		--function-name $(LAMBDA_FUNC_NAME) \
		--auth-type NONE

deploy:
	aws lambda update-function-code \
		--function-name $(LAMBDA_FUNC_NAME) \
		--zip-file fileb://$(PACKAGE_NAME) \
		--region ap-northeast-1
