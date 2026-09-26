package process

import (
	"fmt"
	"log"
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
	httpResp, err := client.R().
		SetContentType("application/json").
		SetResponseExpectContentType("application/json").
		SetBody(req).
		SetResult(&resp).
		Post("/chat/completions")
	if err != nil {
		return resp, err
	}
	if !httpResp.IsStatusSuccess() {
		return resp, fmt.Errorf("chat completion returned HTTP %d", httpResp.StatusCode())
	}
	return resp, err
}

func requestPrompt(client *resty.Client, prompt *types.JebPrompt, maxRetries int) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		response, err := chatCompletionsRequest(client, prompt.Request)
		if err == nil {
			prompt.Response = response
			_, err = JebPromptsToJebResponse([]types.JebPrompt{*prompt})
		}
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < maxRetries {
			log.Printf("retrying %s after attempt %d/%d: %v", prompt.Name, attempt+1, maxRetries+1, err)
		}
	}
	return fmt.Errorf("%s failed after %d attempts: %w", prompt.Name, maxRetries+1, lastErr)
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

			err := requestPrompt(client, &prompts[i], conf.OpenAI.MaxRetries)
			if err != nil {
				mu.Lock()
				if first == nil {
					first = err
				}
				mu.Unlock()
				return
			}
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
