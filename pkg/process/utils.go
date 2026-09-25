package process

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/chand1012/jeb/pkg/calc"
	"github.com/chand1012/jeb/pkg/types"
	"github.com/chand1012/jeb/pkg/utils"
)

func JebPromptsToJebResponse(prompts []types.JebPrompt) (*types.JebResponse, error) {
	jebResponse := &types.JebResponse{Answers: make(map[string]any, len(prompts))}
	for _, prompt := range prompts {
		response := prompt.Response
		// if the prompt response is empty or nil, return an error
		if utils.IsNil(response) {
			return nil, fmt.Errorf("missing response for %s prompt", prompt.Name)
		}
		if len(response.Choices) != 1 {
			return nil, fmt.Errorf("expected one choice for %s response, got %d", prompt.Name, len(response.Choices))
		}
		var labels []string

		answer := response.Choices[0].Message.Content
		if len(response.Choices[0].LogProbs.Content) == 0 {
			return nil, fmt.Errorf("missing token log probabilities for %s response", prompt.Type)
		}

		firstToken := response.Choices[0].LogProbs.Content[0]
		if len(firstToken.TopLogProbs) == 0 {
			return nil, fmt.Errorf("missing top log probabilities for %s response", prompt.Type)
		}

		jebResponse.Usage.InputTokens += response.Usage.PromptTokens
		jebResponse.Usage.OutputTokens += response.Usage.CompletionTokens

		switch prompt.Type {
		case "choice":
			criteria := prompt.Question.Criteria.(map[string]string)
			keys := make([]string, 0, len(criteria))
			for key := range criteria {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			choiceByLabel := make(map[string]string, len(criteria))
			for i, key := range keys {
				label := strconv.Itoa(i)
				labels = append(labels, label)
				choiceByLabel[label] = key
			}
			probs, confidence := calc.Confidence(firstToken.TopLogProbs, labels)
			choice := answer
			if key, ok := choiceByLabel[strings.TrimSpace(firstToken.Token)]; ok {
				choice = key
			}
			jebResponse.Answers[prompt.Name] = types.Choice{
				Type:          "choice",
				Choice:        choice,
				Probabilities: probs,
				Confidence:    confidence,
			}
		case "score":
			var score float64
			criteria := prompt.Question.Criteria.([]string)
			for i := range criteria {
				labels = append(labels, strconv.Itoa(i))
			}
			probs, confidence := calc.Confidence(firstToken.TopLogProbs, labels)
			for label, probability := range probs {
				value, _ := strconv.ParseFloat(label, 64)
				score += value * probability
			}
			legend := make(map[string]string, len(criteria))
			for i, c := range criteria {
				legend[strconv.Itoa(i)] = c
			}

			jebResponse.Answers[prompt.Name] = types.Score{
				Type:          "score",
				Score:         score,
				Legend:        legend,
				Probabilities: probs,
				Confidence:    confidence,
			}
		case "noul":
			noul, err := calc.Noul(answer, firstToken.TopLogProbs)
			if err != nil {
				return nil, err
			}

			jebResponse.Answers[prompt.Name] = types.Noul{
				Type: "noul",
				Noul: noul,
			}
		default:
			return nil, fmt.Errorf("unknown prompt type: %s", prompt.Type)
		}

	}
	return jebResponse, nil
}
