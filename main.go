package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/openai/openai-go"
)

type ChatInput struct {
	Message string `json:"message"`
}

func HandleRequest(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var chatInput ChatInput
	err := json.Unmarshal([]byte(req.Body), &chatInput)
	if err != nil {
		return events.APIGatewayProxyResponse{
			Body:       fmt.Sprintf("Failed to parse input: %s", err.Error()),
			StatusCode: 400,
		}, nil
	}

	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))

	response, _, err := client.ChatCompletion.Create(ctx, &openai.ChatCompletionCreateParams{
		Model: "gpt-3.5-turbo",
		Messages: []*openai.Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: chatInput.Message,
			},
		},
	})

	if err != nil {
		return events.APIGatewayProxyResponse{
			Body:       fmt.Sprintf("Failed to get chat response: %s", err.Error()),
			StatusCode: 500,
		}, nil
	}

	assistantMessage := response.Choices[0].Message.Content

	return events.APIGatewayProxyResponse{
		Body:       assistantMessage,
		StatusCode: 200,
	}, nil
}

func main() {
	lambda.Start(HandleRequest)
}
