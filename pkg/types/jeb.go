package types

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/chand1012/jeb/pkg/config"
	"github.com/chand1012/jeb/pkg/prompts"
	"github.com/chand1012/jeb/pkg/utils"
)

type Question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	// Criteria can either be a map[string]string, a []string, or nil
	// We'll check it at runtime
	Criteria any `json:"criteria"`
}

func (q *Question) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type         string          `json:"type"`
		Instructions string          `json:"instructions"`
		Criteria     json.RawMessage `json:"criteria"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	q.Type = raw.Type
	q.Instructions = raw.Instructions
	switch raw.Type {
	case "choice", "noul":
		if len(raw.Criteria) == 0 || string(raw.Criteria) == "null" {
			q.Criteria = nil
			return nil
		}
		var criteria map[string]string
		if err := json.Unmarshal(raw.Criteria, &criteria); err != nil {
			return fmt.Errorf("decode %s criteria: %w", raw.Type, err)
		}
		q.Criteria = criteria
	case "score":
		var criteria []string
		if err := json.Unmarshal(raw.Criteria, &criteria); err != nil {
			return fmt.Errorf("decode score criteria: %w", err)
		}
		q.Criteria = criteria
	default:
		return fmt.Errorf("invalid question type: %s", raw.Type)
	}
	return nil
}

func (q *Question) GetChoice() (map[string]string, error) {
	if q.Type != "choice" {
		return nil, fmt.Errorf("question type is not choice: %s", q.Type)
	}

	criteria, ok := q.Criteria.(map[string]string)
	if !ok {
		return nil, fmt.Errorf("criteria is not a map[string]string")
	}
	return criteria, nil
}

func (q *Question) GetScore() ([]string, error) {
	if q.Type != "score" {
		return nil, fmt.Errorf("question type is not score: %s", q.Type)
	}

	criteria, ok := q.Criteria.([]string)
	if !ok {
		return nil, fmt.Errorf("criteria is not a []string")
	}
	return criteria, nil
}

func (q *Question) GetNoul() (map[string]string, error) {
	if q.Type != "noul" {
		return nil, fmt.Errorf("question type is not noul: %s", q.Type)
	}

	if q.Criteria == nil {
		return nil, nil
	}

	criteria, ok := q.Criteria.(map[string]string)
	if !ok {
		return nil, fmt.Errorf("criteria is not a map[string]string")
	}
	return criteria, nil
}

type Choice struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

type Score struct {
	Type          string             `json:"type"`
	Score         float64            `json:"score"`
	Legend        map[string]string  `json:"legend"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

type Noul struct {
	Type string  `json:"type"`
	Noul float64 `json:"noul"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type JebPrompt struct {
	Request  ChatCompletionRequest
	Name     string
	Type     string
	Question Question
	Response ChatCompletionsResponse
}

type JebRequest struct {
	State     string              `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// ToJebPrompts converts the JebRequest to a list of ToJebPrompts so we can
// submit them to the OpenAI API
func (r *JebRequest) ToJebPrompts(openaiConfig *config.OpenAIConfig) ([]JebPrompt, error) {
	var requests []JebPrompt

	for name, q := range r.Questions {
		messages := []Message{
			{
				Role:    "system",
				Content: prompts.SystemPrompt,
			},
		}

		options := ""
		responseInstruction := ""

		switch q.Type {
		case "choice":
			responseInstruction = "Reply with only the option number."
			// criteria is a map[string]string
			// first parse it
			criteria := q.Criteria.(map[string]string)
			keys := make([]string, 0, len(criteria))
			for key := range criteria {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			for i, key := range keys {
				options += fmt.Sprintf("%d) %s - %s\n", i, key, criteria[key])
			}
		case "score":
			responseInstruction = "Reply with only the option number."
			// criteria is a []string
			// coerce it then iterate
			criteria := q.Criteria.([]string)
			for i, value := range criteria {
				options += fmt.Sprintf("%d) %s\n", i, value)
			}
		case "noul":
			responseInstruction = "Reply with exactly Yes or No."
			options = "0) Yes"
			if !utils.IsNil(q.Criteria) {
				criteria := q.Criteria.(map[string]string)
				if description, ok := criteria["true"]; ok {
					options += " - " + description
				}
				options += "\n1) No"
				if description, ok := criteria["false"]; ok {
					options += " - " + description
				}
			} else {
				options += "\n1) No"
			}
		default:
			return nil, fmt.Errorf("invalid question type: %s", q.Type)
		}

		messages = append(messages, Message{
			Role:    "user",
			Content: fmt.Sprintf("State: %s\nInstructions: %s\nOptions:\n%s\n%s", r.State, q.Instructions, options, responseInstruction),
		})

		// Top logprobs includes non-option tokens. Request the OpenAI-compatible
		// maximum so those tokens do not crowd valid options out of the result.
		// An option below the top 20 still cannot be measured by this API.
		const topLogProbs = 20

		requests = append(requests, JebPrompt{
			Request: ChatCompletionRequest{
				Model:           openaiConfig.Model,
				Messages:        messages,
				ReasoningEffort: openaiConfig.ReasoningEffort,
				LogProbs:        true,
				TopLogProbs:     topLogProbs,
				Temperature:     openaiConfig.Temperature,
				MaxTokens:       openaiConfig.MaxTokens,
			},
			Name:     name,
			Type:     q.Type,
			Question: q,
		})
	}

	return requests, nil
}

type JebResponse struct {
	Model string `json:"model"`
	// Since answers can be Choice, Score, or Noul, we use any
	Answers map[string]any `json:"answers"`
	Usage   Usage          `json:"usage"`
}
