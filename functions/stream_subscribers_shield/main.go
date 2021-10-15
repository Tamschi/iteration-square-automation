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
)

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {

	// Request validation:

	if request.HTTPMethod != http.MethodGet {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       "Must `GET`.",
		}, nil
	}

	streamName, ok := request.QueryStringParameters["stream"]
	if !ok || streamName == "" {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Query string parameter `stream` missing or empty.",
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

	// Zulip authentication and shared setup:

	zulipUrl.User = url.UserPassword(zulipEmail, zulipApiKey)
	client := &http.Client{}

	// Get stream ID:
	zulipUrl.Path = "api/v1/get_stream_id"

	query := url.Values{}
	query.Set("stream", streamName)
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

	// Get subscribers:
	zulipUrl.Path = "api/v1/streams/" + fmt.Sprint(streamIdResponse.StreamId) + "/members"

	query = url.Values{}
	query.Set("stream", streamName)
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

	type SteamSubscribersResponse struct {
		Msg         string `json:"msg"`
		Result      string `json:"result"`
		Subscribers []int  `json:"subscribers"`
	}
	var streamSubscribersResponse SteamSubscribersResponse
	err = json.Unmarshal(responseBody, &streamSubscribersResponse)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}

	// Badge output:

	// See: <https://shields.io/endpoint>.
	type Shield struct {
		SchemaVersion int    `json:"schemaVersion"`
		Label         string `json:"label"`
		Message       string `json:"message"`
		Color         string `json:"color"`
		NamedLogo     string `json:"namedLogo"`
	}

	badge, err := json.Marshal(Shield{
		SchemaVersion: 1,
		Label:         "chat",
		Message:       fmt.Sprint(len(streamSubscribersResponse.Subscribers)) + " in stream",
		Color:         "g", // Somehow this is a nicer shade than `"green"`.
		NamedLogo:     "zulip",
	})
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}, nil
	}

	return &events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(badge),
	}, nil
}

func main() {
	lambda.Start(handler)
}
