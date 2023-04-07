package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/sashabaranov/go-openai"
	"github.com/slack-go/slack"
)

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	Response string `json:"response"`
}

func HandleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	fmt.Printf("Processing request data for request %s.\n", request.RequestContext.RequestID)
	fmt.Printf("Request headers: %+v.\n", request.Headers)
	fmt.Printf("Request body: %s.\n", request.Body)

	err := config.requestVerifier.Verify(request)
	if err != nil {
		fmt.Printf("Failed to verify: %+v\n", err)
		return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: http.StatusUnauthorized}, nil
	}

	var chatRequest ChatRequest

	err = json.Unmarshal([]byte(request.Body), &chatRequest)
	if err != nil {
		fmt.Printf("Failed to unmarshal JSON for the chat request: %+v\n", err)
		return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: 400}, nil
	}

	response, err := sendToOpenAI(ctx, chatRequest.Message)
	if err != nil {
		fmt.Printf("Failed to request OpenAI: %+v\n", err)
		return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: 500}, nil
	}

	chatResponse := ChatResponse{Response: response}
	responseBody, err := json.Marshal(chatResponse)
	if err != nil {
		fmt.Printf("Failed to marshal JSON for the response body: %+v\n", err)
		return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: 500}, nil
	}

	return events.LambdaFunctionURLResponse{Body: string(responseBody), StatusCode: 200}, nil
}

func sendToOpenAI(ctx context.Context, message string) (string, error) {
	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))

	request := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: message,
			},
		},
	}

	resp, err := client.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

type requestVerifier interface {
	Verify(events.LambdaFunctionURLRequest) error
}

type bearerVerifier struct {
	secret string
}

type slackSignedSecretVerifier struct {
	secret string
}

// Verify the request with slack secret verification
func (v slackSignedSecretVerifier) Verify(r events.LambdaFunctionURLRequest) error {
	header := http.Header{}
	for k, v := range r.Headers {
		header.Set(k, v)
	}

	sv, err := slack.NewSecretsVerifier(header, v.secret)
	if err != nil {
		return err
	}

	sv.Write([]byte(r.Body))
	return sv.Ensure()
}

func (v bearerVerifier) Verify(r events.LambdaFunctionURLRequest) error {
	authHeader := r.Headers["authorization"]
	if authHeader == "" {
		return errors.New("Authorization header is missing")
	}

	headerParts := strings.Split(authHeader, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		return errors.New("Invalid Authorization header format")
	}

	token := headerParts[1]
	if token != v.secret {
		return errors.New("Invalid token")
	}

	return nil
}

var config struct {
	bootMode        string
	requestVerifier requestVerifier
}

func init() {
	config.bootMode = func() string {
		m := os.Getenv("MODE")
		if m == "" {
			m = "dev"
		}
		return m
	}()

	secret := os.Getenv("AUTH_SECRET")
	if secret == "" {
		fmt.Println("Auth secret missing")
		os.Exit(1)
	}

	switch config.bootMode {
	case "dev":
		config.requestVerifier = bearerVerifier{secret: secret}
	case "prod":
		config.requestVerifier = slackSignedSecretVerifier{secret: secret}
	default:
		fmt.Printf("Invalid boot mode: %s\n", config.bootMode)
		os.Exit(1)
	}

	fmt.Printf("Boot mode: %s\n", config.bootMode)
}

func main() {
	debug := os.Getenv("DEBUG")

	if debug != "" {
		req := events.LambdaFunctionURLRequest{Headers: map[string]string{"authorization": "Bearer test-token"}}
		if err := config.requestVerifier.Verify(req); err != nil {
			fmt.Printf("Fail to verify request: %+v", err)
			return
		}

		ctx := context.Background()

		message, err := sendToOpenAI(ctx, "Hello!")
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println(message)

		return
	}

	lambda.Start(HandleRequest)
}
