package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/jxskiss/mcli"
	"resty.dev/v3"
)

const OLLAMA_API_URL = "http://localhost:11434/v1/chat/completions"
const SYSTEM_PROMPT = "You are an expert decision maker! Given the state and some options, please make a choice that best fits the criteria."

type question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	// Criteria can either be a map[string]string, a []string, or nil
	// We'll check it at runtime
	Criteria any `json:"criteria"`
}

func (q *question) UnmarshalJSON(data []byte) error {
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

type jebRequest struct {
	State     string              `json:"state"`
	Questions map[string]question `json:"questions"`
}

type choice struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

type score struct {
	Type          string             `json:"type"`
	Score         float64            `json:"score"`
	Legend        map[string]string  `json:"legend"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

type noul struct {
	Type string  `json:"type"`
	Noul float64 `json:"noul"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type jebResponse struct {
	// Because an answer has three different types, we will evaluate which one it is at runtime
	// Luckily, the type gives us a hint
	Answers map[string]any `json:"answers"`
	Usage   usage          `json:"usage"`
}

type message struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type chatCompletionRequest struct {
	Model           string    `json:"model"`
	Messages        []message `json:"messages"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
	LogProbs        bool      `json:"logprobs,omitempty"`
	TopLogProbs     int       `json:"top_logprobs,omitempty"`
	MaxTokens       int       `json:"max_tokens,omitempty"`
}

type logProbs struct {
	Content []struct {
		Token       string      `json:"token"`
		LogProb     float64     `json:"logprob"`
		TopLogProbs topLogProbs `json:"top_logprobs,omitempty"`
	} `json:"content,omitempty"`
}

type topLogProbs []struct {
	Token   string  `json:"token"`
	LogProb float64 `json:"logprob"`
}

type chatCompletionsResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message  message  `json:"message"`
		LogProbs logProbs `json:"logprobs"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type jebPrompt struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	Messages []message `json:"messages"`
	Question question  `json:"question"`
}

func isNil(i any) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

func genPrompt(request jebRequest) ([]jebPrompt, error) {
	// each question has its own set of messages
	// so we return a list of messages
	var content string
	allPrompts := []jebPrompt{}
	for name, question := range request.Questions {
		messages := []message{{
			Role:    "system",
			Content: SYSTEM_PROMPT,
		}}

		options := ""
		responseInstruction := ""

		switch question.Type {
		case "choice":
			responseInstruction = "Reply with only the option number."
			// criteria is a map[string]string
			// first parse it
			criteria := question.Criteria.(map[string]string)
			i := 1
			for key, value := range criteria {
				options += fmt.Sprintf("%d) %s - %s\n", i, key, value)
				i++
			}
		case "score":
			responseInstruction = "Reply with only the option number."
			// criteria is a []string
			// coerce it then iterate
			criteria := question.Criteria.([]string)
			for i, value := range criteria {
				options += fmt.Sprintf("%d) %s\n", i+1, value)
			}
		case "noul":
			responseInstruction = "Reply with exactly Yes or No."
			// check if criteria is nil
			// if nil, simple options
			if isNil(question.Criteria) {
				options = "1) Yes\n2) No"
			} else {
				// if not nil, use the criteria to generate options.
				// still yes and no, but the input will be labeled as
				// map[string]string where the keys are true and false
				criteria := question.Criteria.(map[string]string)
				for key, value := range criteria {
					k := "Yes"
					if key == "false" {
						k = "No"
					}
					options += fmt.Sprintf("1) %s - %s\n", k, value)
				}
			}
		default:
			return nil, fmt.Errorf("invalid question type: %s", question.Type)
		}

		content = fmt.Sprintf("State: %s\nInstructions: %s\nOptions:\n%s\n%s", request.State, question.Instructions, options, responseInstruction)

		messages = append(messages, message{
			Role:    "user",
			Content: content,
		})

		allPrompts = append(allPrompts, jebPrompt{
			Name:     name,
			Type:     question.Type,
			Messages: messages,
			Question: question,
		})
	}
	return allPrompts, nil
}

func calculateConfidence(
	probs topLogProbs,
	labels []string,
) (map[string]float64, float64) {
	probabilities := make(map[string]float64, len(labels))
	var total float64

	for _, item := range probs {
		token := strings.TrimSpace(item.Token)
		if !slices.Contains(labels, token) {
			continue
		}

		prob := math.Exp(item.LogProb)
		probabilities[token] = prob
		total += prob
	}

	if total == 0 {
		return probabilities, 0
	}

	var confidence float64

	for label, prob := range probabilities {
		normalized := prob / total
		probabilities[label] = normalized
		confidence = max(confidence, normalized)
	}

	return probabilities, confidence
}

func parseResponse(resBody *chatCompletionsResponse, prompt jebPrompt) (any, usage, error) {
	var labels []string

	if len(resBody.Choices) != 1 {
		return nil, usage{}, fmt.Errorf("Incorrect number of choices. Expect 1, got %d", len(resBody.Choices))
	}

	answer := resBody.Choices[0].Message.Content
	if len(resBody.Choices[0].LogProbs.Content) == 0 {
		return nil, usage{}, fmt.Errorf("missing token log probabilities for %s response", prompt.Type)
	}
	firstToken := resBody.Choices[0].LogProbs.Content[0]
	if len(firstToken.TopLogProbs) == 0 {
		return nil, usage{}, fmt.Errorf("missing top token log probabilities for %s response", prompt.Type)
	}
	returnedUsage := usage{
		InputTokens:  resBody.Usage.PromptTokens,
		OutputTokens: resBody.Usage.CompletionTokens,
	}

	switch prompt.Type {
	case "choice":
		// we don't care about the criteria themselves, just their length
		// since we use numbers for labels we can generate a list of labels
		// from the length of the criteria
		// first we need to coerce the criteria into a map
		criteria := prompt.Question.Criteria.(map[string]string)
		for i := 1; i <= len(criteria); i++ {
			labels = append(labels, strconv.Itoa(i))
		}

		probs, confidence := calculateConfidence(firstToken.TopLogProbs, labels)
		return choice{
			Type:          prompt.Type,
			Choice:        answer,
			Probabilities: probs,
			Confidence:    confidence,
		}, returnedUsage, nil
	case "score":
		var totalScore float64
		// similar to above, we just need to coerce it so we can get the length
		criteria := prompt.Question.Criteria.([]string)
		for i := 1; i <= len(criteria); i++ {
			labels = append(labels, strconv.Itoa(i))
		}

		probs, confidence := calculateConfidence(firstToken.TopLogProbs, labels)
		for label, probability := range probs {
			value, _ := strconv.ParseFloat(label, 64)
			totalScore += value * probability
		}

		legend := make(map[string]string, len(criteria))

		for i, c := range criteria {
			legend[strconv.Itoa(i+1)] = c
		}

		return score{
			Type:          "score",
			Score:         totalScore,
			Probabilities: probs,
			Confidence:    confidence,
			Legend:        legend,
		}, returnedUsage, nil

	case "noul":
		var noProbability float64
		hasNo := false
		for _, candidate := range firstToken.TopLogProbs {
			switch strings.TrimSpace(candidate.Token) {
			case "Yes":
				return noul{
					Noul: math.Exp(candidate.LogProb),
					Type: "noul",
				}, returnedUsage, nil
			case "No":
				noProbability = math.Exp(candidate.LogProb)
				hasNo = true
			}
		}
		if strings.TrimSpace(answer) == "No" && hasNo {
			return noul{
				Noul: 1 - noProbability,
				Type: "noul",
			}, returnedUsage, nil
		}
		return nil, usage{}, fmt.Errorf("cannot determine Yes probability from noul response")
	default:
		return nil, usage{}, fmt.Errorf("invalid prompt type: %s", prompt.Type)
	}
}

func req(request jebRequest, model string) (jebResponse, error) {
	allPrompts, err := genPrompt(request)
	if err != nil {
		return jebResponse{}, err
	}

	totalUsage := usage{
		InputTokens:  0,
		OutputTokens: 0,
	}
	answers := make(map[string]any, len(allPrompts))

	client := resty.New()
	defer client.Close()

	// eventually we'll parallelize this
	// for now just make it a loop of requests.
	for _, prompt := range allPrompts {
		body := chatCompletionRequest{
			Model:           model,
			Messages:        prompt.Messages,
			ReasoningEffort: "none",
			LogProbs:        true,
			TopLogProbs:     3,
			MaxTokens:       10,
		}
		resBody := &chatCompletionsResponse{}
		_, err := client.R().
			SetContentType("application/json").
			SetResponseExpectContentType("application/json").
			SetBody(body).
			SetResult(resBody).
			Post(OLLAMA_API_URL)
		if err != nil {
			return jebResponse{}, fmt.Errorf("request question %q: %w", prompt.Name, err)
		}

		answer, use, err := parseResponse(resBody, prompt)
		if err != nil {
			return jebResponse{}, err
		}
		answers[prompt.Name] = answer

		// add the usage to the total
		totalUsage.InputTokens += use.InputTokens
		totalUsage.OutputTokens += use.OutputTokens
	}
	return jebResponse{Answers: answers, Usage: totalUsage}, nil
}

func serve() {
	var args struct {
		Host  string `cli:"-h, --host, which network interface are we running on" default:"127.0.0.1"`
		Port  int    `cli:"-p, --port, which port are we running on" default:"6102"`
		Model string `cli:"-m, --model, default model to send" default:"qwen3.5:9b"`
	}

	mcli.Parse(&args)

	url := fmt.Sprintf("http://%s:%d/v1", args.Host, args.Port)

	fmt.Println("I'm serving on", url, "model", args.Model)

	// Your server logic here
	fmt.Println("Server is running")
}

func runRequest(input io.Reader, output io.Writer, model string, requestFn func(jebRequest, string) (jebResponse, error)) error {
	decoder := json.NewDecoder(input)
	var request jebRequest
	if err := decoder.Decode(&request); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("expected one JSON request")
		}
		return fmt.Errorf("decode trailing input: %w", err)
	}

	response, err := requestFn(request, model)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(response)
}

func run() {
	var args struct {
		Model string `cli:"-m, --model, model to send" default:"qwen3.5:9b"`
	}
	mcli.Parse(&args)
	if err := runRequest(os.Stdin, os.Stdout, args.Model, req); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	mcli.AddRoot(run)
	mcli.Add("serve", serve, "Run the jeb server")
	mcli.Run()
}
