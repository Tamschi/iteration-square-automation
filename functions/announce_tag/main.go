package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/google/go-github/v39/github"

	"github.com/reecerussell/aws-lambda-multipart-parser/parser"

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

	data, err := parser.Parse(request)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}

	project, ok := data.Get("project")
	if !ok || project == "" {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Multipart-form parameter `project` missing or empty.",
		}, nil
	}

	tag, ok := data.Get("tag")
	if !ok || tag == "" {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Multipart-form parameter `tag` missing or empty.",
		}, nil
	}

	//  Zulip config validation:

	zulipApiKey, ok := os.LookupEnv("TAG_ANNOUNCEMENT_BOT_ZULIP_API_KEY")
	if !ok {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "`ZULIP_API_KEY` not set.",
		}, nil
	}

	zulipApiUrl, ok := os.LookupEnv("ZULIP_API_URL")
	if !ok {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "`ZULIP_API_URL` not set.",
		}, nil
	}

	zulipUrl, err := url.Parse(zulipApiUrl)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}

	zulipEmail, ok := os.LookupEnv("TAG_ANNOUNCEMENT_BOT_ZULIP_EMAIL")
	if !ok {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "`ZULIP_EMAIL` not set.",
		}, nil
	}

	// Repository validation:

	githubTs := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: github_token},
	)
	githubTc := oauth2.NewClient(ctx, githubTs)

	githubClient := github.NewClient(githubTc)
	repository, res, err := githubClient.Repositories.Get(ctx, "Tamschi", project)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: res.StatusCode,
			Body:       err.Error(),
		}, nil
	}

	// Zulip authentication and shared setup:

	zulipUrl.User = url.UserPassword(zulipEmail, zulipApiKey)
	client := &http.Client{}

	// Announce tag:
	zulipUrl.Path = "api/v1/messages"

	query := url.Values{}
	query.Set("type", "stream")

	to, err := json.Marshal([]string{"project/" + *repository.Name})
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}

	query.Set("to", string(to))
	query.Set("topic", "tag announcements")
	query.Set("content", `Tag pushed: [`+tag+`](https://github.com/Tamschi/`+*repository.Name+`/releases/tag/`+tag+`)`)
	zulipUrl.RawQuery = query.Encode()

	zulipRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, zulipUrl.String(), nil)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}

	response, err := client.Do(zulipRequest)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}
	if 200 > response.StatusCode || response.StatusCode >= 300 {
		return &events.APIGatewayProxyResponse{
			StatusCode: response.StatusCode,
			Body:       zulipUrl.Redacted() + "\n\n" + string(responseBody),
		}, nil
	}

	// ---

	return &events.APIGatewayProxyResponse{
		StatusCode: response.StatusCode,
		Body:       "Announced tag.",
	}, nil
}

func main() {
	lambda.Start(handler)
}
