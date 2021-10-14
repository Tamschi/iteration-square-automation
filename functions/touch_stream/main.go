package main

import (
	"context"
	"errors"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"
)

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	github_ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: "..."}, //TODO
	)
	github_tc := oauth2.NewClient(ctx, github_ts)

	github_client := github.NewClient(github_tc)
	repository, _, err := github_client.Repositories.Get(ctx, "Tamschi", "rust-template") //TODO
	if err != nil {
		return nil, err
	}

	_ = repository

	return nil, errors.New("Not implemented")
}

func main() {
	lambda.Start(handler)
}
