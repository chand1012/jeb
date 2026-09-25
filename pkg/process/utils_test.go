package process

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/chand1012/jeb/pkg/types"
)

func TestJebPromptsToJebResponseUsesZeroBasedLabels(t *testing.T) {
	response := func(token string) types.ChatCompletionsResponse {
		t.Helper()
		var result types.ChatCompletionsResponse
		data := `{"choices":[{"message":{"role":"assistant","content":"` + token + `"},"logprobs":{"content":[{"token":"` + token + `","logprob":-0.6931471805599453,"top_logprobs":[{"token":"0","logprob":-0.6931471805599453},{"token":"1","logprob":-0.6931471805599453}]}]}}]}`
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	prompts := []types.JebPrompt{
		{Name: "choice", Type: "choice", Question: types.Question{Criteria: map[string]string{"apple": "first", "zebra": "last"}}, Response: response("0")},
		{Name: "score", Type: "score", Question: types.Question{Criteria: []string{"low", "high"}}, Response: response("1")},
	}
	got, err := JebPromptsToJebResponse(prompts)
	if err != nil {
		t.Fatal(err)
	}
	choice := got.Answers["choice"].(types.Choice)
	if choice.Choice != "apple" || choice.Probabilities["0"] != 0.5 || choice.Probabilities["1"] != 0.5 {
		t.Errorf("choice = %+v", choice)
	}
	score := got.Answers["score"].(types.Score)
	if score.Legend["0"] != "low" || score.Legend["1"] != "high" || score.Probabilities["0"] != 0.5 || score.Probabilities["1"] != 0.5 || math.Abs(score.Score-0.5) > 1e-12 {
		t.Errorf("score = %+v", score)
	}
}
