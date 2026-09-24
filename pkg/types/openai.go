package types

type Message struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type ChatCompletionRequest struct {
	Model           string    `json:"model"`
	Messages        []Message `json:"messages"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
	LogProbs        bool      `json:"logprobs,omitempty"`
	TopLogProbs     int       `json:"top_logprobs,omitempty"`
	Temperature     *float64  `json:"temperature,omitempty"`
	MaxTokens       int       `json:"max_tokens,omitempty"`
}

type LogProbs struct {
	Content []struct {
		Token       string      `json:"token"`
		LogProb     float64     `json:"logprob"`
		TopLogProbs TopLogProbs `json:"top_logprobs,omitempty"`
	} `json:"content,omitempty"`
}

type TopLogProbs []struct {
	Token   string  `json:"token"`
	LogProb float64 `json:"logprob"`
}

type ChatCompletionsResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message  Message  `json:"message"`
		LogProbs LogProbs `json:"logprobs"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
