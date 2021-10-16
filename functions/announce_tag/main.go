package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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

	project, ok := request.QueryStringParameters["project"]
	if !ok || project == "" {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Query string parameter `project` missing or empty.",
		}, nil
	}

	tag, ok := request.QueryStringParameters["tag"]
	if !ok || tag == "" {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Query string parameter `tag` missing or empty.",
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

	// Get stream ID:
	zulipUrl.Path = "api/v1/get_stream_id"

	query := url.Values{}
	query.Set("stream", "project/"+project)
	zulipUrl.RawQuery = query.Encode()

	zulipRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, zulipUrl.String(), nil)
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

	type SteamIdResponse struct {
		Msg      string `json:"msg"`
		Result   string `json:"result"`
		StreamId int    `json:"stream_id"`
	}
	var streamIdResponse SteamIdResponse
	err = json.Unmarshal(responseBody, &streamIdResponse)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}

	// Announce tag:
	zulipUrl.Path = "api/v1/messages"

	query = url.Values{}
	query.Set("type", "stream")
	query.Set("to", "["+fmt.Sprint(streamIdResponse.StreamId)+"]")
	query.Set("topic", "tag announcements")
	query.Set("content", `Tag pushed: [`+tag+`](https://github.com/Tamschi/`+*repository.Name+`/releases/tag/`+tag+`)`)
	zulipUrl.RawQuery = query.Encode()

	zulipRequest, err = http.NewRequestWithContext(ctx, http.MethodPost, zulipUrl.String(), nil)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}

	response, err = client.Do(zulipRequest)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}

	responseBody, err = ioutil.ReadAll(response.Body)
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
