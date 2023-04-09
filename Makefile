.PHONY: build package clean

BINARY_NAME := main
PACKAGE_NAME := deployment.zip
LAMBDA_FUNC_NAME := OpenAIChatSlackBot

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
	aws lambda update-function-configuration --function-name $(LAMBDA_FUNC_NAME) \
		--environment "Variables={MODE=prod,AUTH_SECRET=xxx,OPENAI_API_KEY=xxx,SLACK_API_TOKEN=xxx}"

# Requires env LAMBDA_ROLE_ARN
provision-dev:
	aws lambda create-function \
		--function-name $(LAMBDA_FUNC_NAME)Dev \
		--runtime go1.x \
		--role $(LAMBDA_ROLE_ARN) \
		--handler $(BINARY_NAME) \
		--zip-file fileb://$(PACKAGE_NAME) \
		--region ap-northeast-1
	aws lambda create-function-url-config \
		--function-name $(LAMBDA_FUNC_NAME)Dev \
		--auth-type NONE
	aws lambda update-function-configuration --function-name $(LAMBDA_FUNC_NAME)Dev \
		--environment "Variables={MODE=dev,AUTH_SECRET=xxx,OPENAI_API_KEY=xxx,SLACK_API_TOKEN=xxx}"

deploy:
	aws lambda update-function-code \
		--function-name $(LAMBDA_FUNC_NAME) \
		--zip-file fileb://$(PACKAGE_NAME) \
		--region ap-northeast-1

deploy-dev:
	aws lambda update-function-code \
		--function-name $(LAMBDA_FUNC_NAME)Dev \
		--zip-file fileb://$(PACKAGE_NAME) \
		--region ap-northeast-1
