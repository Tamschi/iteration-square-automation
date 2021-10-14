package main

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/google/go-github/v39/github"

	"golang.org/x/oauth2"
)

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {

	// Request validation:

	if request.HTTPMethod != http.MethodPost {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       "Must `POST`.",
		}, nil
	}

	github_token, ok := request.Headers["authorization"]
	if !ok {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       "`Authorization` header missing.",
		}, nil
	}

	repository_name := request.Body
	if request.Body == "" {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Empty request body/repository name.",
		}, nil
	}

	//  Zulip config validation:

	zulipAPIKey, ok := os.LookupEnv("ZULIP_API_KEY")
	if !ok {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "`ZULIP_API_KEY` not set.",
		}, nil
	}

	zulipAPIURL, ok := os.LookupEnv("ZULIP_API_URL")
	if !ok {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "`ZULIP_API_URL` not set.",
		}, nil
	}

	zulipEmail, ok := os.LookupEnv("ZULIP_EMAIL")
	if !ok {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "`ZULIP_EMAIL` not set.",
		}, nil
	}

	_, _, _ = zulipAPIKey, zulipAPIURL, zulipEmail

	// Repository validation:

	githubTs := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: github_token},
	)
	githubTc := oauth2.NewClient(ctx, githubTs)

	githubClient := github.NewClient(githubTc)
	repository, res, err := githubClient.Repositories.Get(ctx, "Tamschi", repository_name)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: res.StatusCode,
			Body:       err.Error(),
		}, nil
	}

	// Touch stream:

	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}

	_ = repository

	return nil, errors.New("Not implemented.")
}

func main() {
	lambda.Start(handler)
}
