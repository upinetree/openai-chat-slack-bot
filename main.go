package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/sashabaranov/go-openai"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/upinetree/openai-chat-slack-bot/log"
)

func HandleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	log.Info("Request headers: %+v", request.Headers)
	log.Info("Request body: %s", request.Body)

	// Simply avoid retries, generally for the 3 seconds limit
	// https://api.slack.com/apis/connections/events-api#retries
	if request.Headers["x-slack-retry-num"] != "" {
		log.Error("Avoid retries (%+v): %+v", request.Headers["x-slack-retry-num"], request.Headers["x-slack-retry-reason"])
		return events.LambdaFunctionURLResponse{StatusCode: http.StatusOK}, nil
	}

	err := config.requestVerifier.Verify(request)
	if err != nil {
		log.Error("Failed to verify: %+v", err)
		return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: http.StatusUnauthorized}, nil
	}

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(request.Body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Error("Failed to parse request body as a slack event: %+v", err)
		return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: http.StatusInternalServerError}, nil
	}

	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(request.Body), &r)
		if err != nil {
			log.Error("Failed to unmarshal JSON as a slack challenge response: %+v", err)
			return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: http.StatusBadRequest}, nil
		}
		return events.LambdaFunctionURLResponse{Body: r.Challenge, StatusCode: 200}, nil
	}

	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			m, err := removeSlackMention(ev.Text)
			if err != nil {
				log.Error("Failed to remove slack mention from event text: %+v", err)
				return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: http.StatusInternalServerError}, nil
			}

			log.Info("Requesting OpenAI API: %s", m)
			response, err := sendToOpenAI(ctx, m)
			if err != nil {
				log.Error("Failed to request OpenAI: %+v", err)
				return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: http.StatusInternalServerError}, nil
			}

			api := slack.New(config.slackAPIToken)
			_, _, err = api.PostMessage(ev.Channel, slack.MsgOptionText(response, false))
			if err != nil {
				log.Error("Failed to send a response message to slack: %+v", err)
				return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: http.StatusBadRequest}, nil
			}
		}
	}

	// Include the message in the response, assuming it is not a slack request (generally for dev)
	if eventsAPIEvent.Type == "" {
		var chatRequest struct {
			Message string `json:"message"`
		}

		err = json.Unmarshal([]byte(request.Body), &chatRequest)
		if err != nil {
			log.Error("Failed to unmarshal JSON for the chat request: %+v", err)
			return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: 400}, nil
		}

		log.Info("Requesting OpenAI API: %s", chatRequest.Message)
		response, err := sendToOpenAI(ctx, chatRequest.Message)
		if err != nil {
			log.Error("Failed to request OpenAI: %+v", err)
			return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: 500}, nil
		}

		chatResponse := struct {
			Response string `json:"response"`
		}{Response: response}

		responseBody, err := json.Marshal(chatResponse)
		if err != nil {
			log.Error("Failed to marshal JSON for the response body: %+v", err)
			return events.LambdaFunctionURLResponse{Body: err.Error(), StatusCode: 500}, nil
		}

		return events.LambdaFunctionURLResponse{Body: string(responseBody), StatusCode: 200}, nil
	}

	return events.LambdaFunctionURLResponse{StatusCode: http.StatusOK}, nil
}

func removeSlackMention(m string) (string, error) {
	pattern := "<@[A-Za-z0-9]+>"
	replaceWith := ""

	re, err := regexp.Compile(pattern)
	if err != nil {
		return m, err
	}

	return re.ReplaceAllString(m, replaceWith), nil
}

func sendToOpenAI(ctx context.Context, message string) (string, error) {
	client := openai.NewClient(config.openAIAPIKey)

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
	openAIAPIKey    string
	slackAPIToken   string
}

func init() {
	config.bootMode = func() string {
		m := os.Getenv("MODE")
		if m == "" {
			m = "dev"
		}
		return m
	}()
	log.Info("Boot mode: %s", config.bootMode)

	secret := os.Getenv("AUTH_SECRET")
	if secret == "" {
		log.Error("Auth secret missing")
		os.Exit(1)
	}

	switch config.bootMode {
	case "dev":
		config.requestVerifier = bearerVerifier{secret: secret}
	case "prod":
		config.requestVerifier = slackSignedSecretVerifier{secret: secret}
	default:
		log.Error("Invalid boot mode: %s", config.bootMode)
		os.Exit(1)
	}

	config.openAIAPIKey = os.Getenv("OPENAI_API_KEY")
	if config.openAIAPIKey == "" {
		log.Error("OpenAI API Key is missing")
		os.Exit(1)
	}

	config.slackAPIToken = os.Getenv("SLACK_API_TOKEN")
	if config.slackAPIToken == "" {
		log.Error("Slack API Token is missing")
		os.Exit(1)
	}
}

func main() {
	debug := os.Getenv("DEBUG")

	if debug != "" {
		req := events.LambdaFunctionURLRequest{Headers: map[string]string{"authorization": "Bearer test-token"}}
		if err := config.requestVerifier.Verify(req); err != nil {
			log.Error("Failed to verify request: %+v", err)
			return
		}

		ctx := context.Background()

		message, err := sendToOpenAI(ctx, "Hello!")
		if err != nil {
			log.Error("Failed to send a message to OpenAI API", err)
			return
		}

		log.Info(message)

		{
			api := slack.New(config.slackAPIToken)
			_, _, err := api.PostMessage(os.Getenv("DEBUG_SLACK_CH_ID"), slack.MsgOptionText(message, false))
			if err != nil {
				log.Warn("Failed to send a response message to slack: %+v", err)
			}
		}

		return
	}

	lambda.Start(HandleRequest)
}
