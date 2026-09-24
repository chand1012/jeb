package process

import (
	"sync"

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

func chatCompletionsRequest(client *resty.Client, req types.ChatCompletionRequest) (types.ChatCompletionsResponse, error) {
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

	// resty clients are safe for concurrent use, so create one for all prompts.
	client := newClient(&conf.OpenAI)
	defer client.Close()

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		first error
	)
	sem := make(chan struct{}, conf.Concurrency.MaxRequests)

	for i := range prompts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := chatCompletionsRequest(client, prompts[i].Request)
			if err != nil {
				mu.Lock()
				if first == nil {
					first = err
				}
				mu.Unlock()
				return
			}
			prompts[i].Response = res
		}(i)
	}
	wg.Wait()
	if first != nil {
		return types.JebResponse{}, first
	}

	responses, err := JebPromptsToJebResponse(prompts)
	if err != nil {
		return types.JebResponse{}, err
	}
	responses.Model = conf.OpenAI.Model
	return *responses, nil
}
