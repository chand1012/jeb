package calc

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/chand1012/jeb/pkg/types"
)

func Confidence(
	probs types.TopLogProbs,
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

func Noul(answer string, probs types.TopLogProbs) (float64, error) {
	var noProbability float64
	hasNo := false
	for _, candidate := range probs {
		switch strings.TrimSpace(candidate.Token) {
		case "Yes":
			return math.Exp(candidate.LogProb), nil
		case "No":
			noProbability = math.Exp(candidate.LogProb)
			hasNo = true
		}
	}
	if strings.TrimSpace(answer) == "No" && hasNo {
		return 1 - noProbability, nil
	}
	return 0, fmt.Errorf("cannot determine Yes probability from given logprobs")
}
