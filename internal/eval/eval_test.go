package eval

import (
	"context"
	"os"
	"testing"
)

func TestHistoricalCorpus(t *testing.T) {
	data, err := os.ReadFile("../../evals/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	report, err := Run(context.Background(), data, nil, "", "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range report.Results {
		if row.Variant == "after" && !row.Exact {
			t.Errorf("%s: got %q, want %q; error=%s", row.ID, row.Got, row.Want, row.Error)
		}
	}
}
