package process

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chand1012/jeb/pkg/config"
	"github.com/chand1012/jeb/pkg/types"
)

func choiceCompletion(token string) string {
	return fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":%q},"logprobs":{"content":[{"token":%q,"top_logprobs":[{"token":"0","logprob":-0.1},{"token":"1","logprob":-2.3}]}]}}]}`, token, token)
}

func TestRequestPromptRetriesHTTPAndMalformedChoice(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch requests {
		case 1:
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprint(w, `{"error":"temporary"}`)
		case 2:
			fmt.Fprint(w, choiceCompletion("2"))
		default:
			fmt.Fprint(w, choiceCompletion("0"))
		}
	}))
	defer server.Close()

	client := newClient(&config.OpenAIConfig{BaseURL: server.URL})
	defer client.Close()
	prompt := types.JebPrompt{
		Name: "decision", Type: "choice",
		Question: types.Question{Criteria: map[string]string{"apple": "first", "zebra": "second"}},
	}
	if err := requestPrompt(client, &prompt, 3); err != nil {
		t.Fatal(err)
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
	got, err := JebPromptsToJebResponse([]types.JebPrompt{prompt})
	if err != nil {
		t.Fatal(err)
	}
	if got.Answers["decision"].(types.Choice).Choice != "apple" {
		t.Fatalf("answer = %+v", got.Answers["decision"])
	}
}

func TestRequestPromptStopsAfterThreeRetries(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, choiceCompletion("2"))
	}))
	defer server.Close()

	client := newClient(&config.OpenAIConfig{BaseURL: server.URL})
	defer client.Close()
	prompt := types.JebPrompt{
		Name: "decision", Type: "choice",
		Question: types.Question{Criteria: map[string]string{"apple": "first", "zebra": "second"}},
	}
	err := requestPrompt(client, &prompt, 3)
	if requests != 4 || err == nil || !strings.Contains(err.Error(), "after 4 attempts") {
		t.Fatalf("requests = %d, err = %v", requests, err)
	}
}
