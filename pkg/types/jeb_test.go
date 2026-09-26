package types

import (
	"strings"
	"testing"

	"github.com/chand1012/jeb/pkg/config"
)

func TestToJebPromptsUsesZeroBasedOptions(t *testing.T) {
	req := JebRequest{
		State: "test state",
		Questions: map[string]Question{
			"choice":     {Type: "choice", Criteria: map[string]string{"zebra": "last", "apple": "first"}},
			"score":      {Type: "score", Criteria: []string{"low", "medium", "high"}},
			"noul":       {Type: "noul", Criteria: map[string]string{"true": "approve", "false": "deny"}},
			"plain_noul": {Type: "noul"},
		},
	}
	prompts, err := req.ToJebPrompts(&config.OpenAIConfig{Model: "test"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"choice":     "Options:\n0) apple - first\n1) zebra - last\n",
		"score":      "Options:\n0) low\n1) medium\n2) high\n",
		"noul":       "Options:\n0) Yes - approve\n1) No - deny\n",
		"plain_noul": "Options:\n0) Yes\n1) No\n",
	}
	for _, prompt := range prompts {
		got := prompt.Request.Messages[1].Content
		if !strings.Contains(got, want[prompt.Name]) {
			t.Errorf("%s prompt lacks zero-based options: %q", prompt.Name, got)
		}
		if prompt.Name == "choice" {
			if !strings.Contains(got, "exactly one number from 0 through 1") || !strings.Contains(got, "Do not use 1-based numbering") || !strings.Contains(got, "Do not include an option name, words, reasoning, punctuation, or any other text") {
				t.Errorf("choice prompt lacks strict numeric output instruction: %q", got)
			}
			if !strings.Contains(prompt.Request.Messages[0].Content, "output exactly one listed option number and nothing else") {
				t.Errorf("choice system prompt lacks strict numeric output instruction: %q", prompt.Request.Messages[0].Content)
			}
		}
		if prompt.Request.TopLogProbs != 20 {
			t.Errorf("%s requested %d top logprobs, want 20 to include options behind non-option tokens", prompt.Name, prompt.Request.TopLogProbs)
		}
	}
}
