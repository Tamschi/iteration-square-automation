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

	repositoryName := request.Body
	if request.Body == "" {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Empty request body/repository name.",
		}, nil
	}

	//  Zulip config validation:

	zulipApiKey, ok := os.LookupEnv("PROJECT_STREAM_BOT_ZULIP_API_KEY")
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

	zulipEmail, ok := os.LookupEnv("PROJECT_STREAM_BOT_ZULIP_EMAIL")
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
	repository, res, err := githubClient.Repositories.Get(ctx, "Tamschi", repositoryName)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: res.StatusCode,
			Body:       err.Error(),
		}, nil
	}

	// Zulip authentication and shared setup:

	zulipUrl.User = url.UserPassword(zulipEmail, zulipApiKey)
	client := &http.Client{}

	// Touch stream:
	zulipUrl.Path = "api/v1/users/me/subscriptions"

	query := url.Values{}

	type subscription struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	subscriptions, err := json.Marshal([]subscription{{"project/" + *repository.Name, *repository.Description}})
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}
	query.Set("subscriptions", string(subscriptions))

	// user450752@iter-square.zulipchat.com // Tamme's Zulip mail. Ideally the bot would grab the list from the "core team" channel, though.
	principals, err := json.Marshal([]int{450752})
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}
	query.Set("principals", string(principals))

	query.Set("authorization_errors_fatal", "true")
	query.Set("announce", "true")
	// query.Set("is_web_public", "true") // Not enabled for the free organisations available by default.
	query.Set("history_public_to_subscribers", "true")
	query.Set("stream_post_policy", "1") // Any user can post.
	query.Set("message_retention_days", "\"realm_default\"")

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

	// Get stream ID:
	zulipUrl.Path = "api/v1/get_stream_id"

	query = url.Values{}
	query.Set("stream", "project/" + repositoryName)
	zulipUrl.RawQuery = query.Encode()

	zulipRequest, err = http.NewRequestWithContext(ctx, http.MethodGet, zulipUrl.String(), nil)
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

	// Update stream:
	zulipUrl.Path = "api/v1/streams/" + fmt.Sprint(streamIdResponse.StreamId)

	query = url.Values{}

	description, err := json.Marshal(repository.Description)
	query.Set("description", string(description))
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}
	zulipUrl.RawQuery = query.Encode()

	zulipRequest, err = http.NewRequestWithContext(ctx, http.MethodPatch, zulipUrl.String(), nil)
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
		Body:       "Created stream or updated stream description.",
	}, nil
}

func main() {
	lambda.Start(handler)
}
