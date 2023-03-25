package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/sashabaranov/go-openai"
)

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	Response string `json:"response"`
}

func HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	fmt.Printf("Processing request data for request %s.\n", request.RequestContext.RequestID)

	var chatRequest ChatRequest

	err := json.Unmarshal([]byte(request.Body), &chatRequest)
	if err != nil {
		return events.APIGatewayProxyResponse{Body: err.Error(), StatusCode: 400}, nil
	}

	response, err := sendToOpenAI(ctx, chatRequest.Message)
	if err != nil {
		return events.APIGatewayProxyResponse{Body: err.Error(), StatusCode: 500}, nil
	}

	chatResponse := ChatResponse{Response: response}
	responseBody, err := json.Marshal(chatResponse)
	if err != nil {
		return events.APIGatewayProxyResponse{Body: err.Error(), StatusCode: 500}, nil
	}

	return events.APIGatewayProxyResponse{Body: string(responseBody), StatusCode: 200}, nil
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

func main() {
	debug := os.Getenv("DEBUG")

	if debug != "" {
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
