package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/google/go-github/v39/github"

	"golang.org/x/oauth2"
)

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {

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

	github_ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: github_token},
	)
	github_tc := oauth2.NewClient(ctx, github_ts)

	github_client := github.NewClient(github_tc)
	repository, res, err := github_client.Repositories.Get(ctx, "Tamschi", repository_name)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: res.StatusCode,
			Body:       err.Error(),
		}, nil
	}

	_ = repository

	return nil, errors.New("Not implemented.")
}

func main() {
	lambda.Start(handler)
}
