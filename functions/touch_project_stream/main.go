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

	zulipApiKey, ok := os.LookupEnv("ZULIP_API_KEY")
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

	zulipEmail, ok := os.LookupEnv("ZULIP_EMAIL")
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
	repository, res, err := githubClient.Repositories.Get(ctx, "Tamschi", repository_name)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: res.StatusCode,
			Body:       err.Error(),
		}, nil
	}

	// Touch stream:
	zulipUrl.Path = "api/v1/users/me/subscriptions"

	zulipUrl.User = url.UserPassword(zulipEmail, zulipApiKey)

	query, err := url.ParseQuery(zulipUrl.RawQuery)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: res.StatusCode,
			Body:       err.Error(),
		}, nil
	}

	type subscription struct {
		name        string
		description string
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
	query.Set("is_web_public", "true")
	query.Set("history_public_to_subscribers", "true")
	query.Set("stream_post_policy", "1")                 // Any user can post.
	query.Set("message_retention_days", "realm_default") // Any user can post.

	zulipUrl.RawQuery = query.Encode()
	zulipRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, zulipApiUrl, nil)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}

	zulipRequest.Header.Add("Referer", "https://" + zulipUrl.Host + "/")

	client := &http.Client{}
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

	return &events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Response from Zulip:\n\n" + string(responseBody), // What. No validation?
	}, nil
}

func main() {
	lambda.Start(handler)
}
