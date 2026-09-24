package process

import (
	"github.com/chand1012/jeb/pkg/config"
	"github.com/chand1012/jeb/pkg/types"
	"resty.dev/v3"
)

func newClient(openaiConfig *config.OpenAIConfig) *resty.Client {
	client := resty.New()
	client.SetBaseURL(openaiConfig.BaseURL)
	client.SetTimeout(openaiConfig.Timeout)
	if openaiConfig.APIKey != "" {
		client.SetHeader("Authorization", "Bearer "+openaiConfig.APIKey)
	}
	return client
}

func chatCompletionsRequest(req types.ChatCompletionRequest, openaiConfig *config.OpenAIConfig) (types.ChatCompletionsResponse, error) {
	client := newClient(openaiConfig)
	defer client.Close()

	var resp types.ChatCompletionsResponse
	_, err := client.R().
		SetContentType("application/json").
		SetResponseExpectContentType("application/json").
		SetBody(req).
		SetResult(&resp).
		Post("/chat/completions")
	return resp, err
}

func Request(req types.JebRequest, conf *config.Config) (types.JebResponse, error) {
	prompts, err := req.ToJebPrompts(&conf.OpenAI)
	if err != nil {
		return types.JebResponse{}, err
	}

	// will parallelize this later
	for i := 0; i < len(prompts); i++ {
		res, err := chatCompletionsRequest(prompts[i].Request, &conf.OpenAI)
		if err != nil {
			return types.JebResponse{}, err
		}
		prompts[i].Response = res
	}

	responses, err := JebPromptsToJebResponse(prompts)
	if err != nil {
		return types.JebResponse{}, err
	}
	return *responses, nil
}
