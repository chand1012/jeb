package calc

import (
	"math"
	"testing"

	"github.com/chand1012/jeb/pkg/types"
)

func TestNoulConditionsOnYesAndNo(t *testing.T) {
	probs := types.TopLogProbs{
		{Token: "No", LogProb: math.Log(0.6)},
		{Token: "Other", LogProb: math.Log(0.1)},
		{Token: "Yes", LogProb: math.Log(0.3)},
	}
	got, err := Noul("No", probs)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-1.0/3.0) > 1e-12 {
		t.Errorf("Yes probability = %v, want 1/3 conditional on Yes or No", got)
	}
}

func TestNoulDoesNotAssignUnobservedMassToYes(t *testing.T) {
	probs := types.TopLogProbs{{Token: "No", LogProb: math.Log(0.6)}}
	if _, err := Noul("No", probs); err == nil {
		t.Fatal("expected error when Yes is absent from top logprobs")
	}
}
