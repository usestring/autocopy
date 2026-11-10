package eval

import (
	"context"
	"os"
	"testing"

	"github.com/usestring/autocopy"
)

func TestHistoricalCorpus(t *testing.T) {
	data, err := os.ReadFile("../../evals/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	report, err := Run(context.Background(), data, nil, "", "test", Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range report.Results {
		if row.Variant == "after" && !row.Exact {
			t.Errorf("%s: got %q, want %q; error=%s", row.ID, row.Got, row.Want, row.Error)
		}
	}
}

type classifierFunc func(context.Context, autocopy.Input) (map[string]float64, error)

func (f classifierFunc) Classify(ctx context.Context, input autocopy.Input) (map[string]float64, error) {
	return f(ctx, input)
}

func TestSelectedCurrentCasesLimitRequests(t *testing.T) {
	data := []byte(`[
		{"id":"selected","message":"Run ` + "`selected command`" + `.","want":["selected command"]},
		{"id":"excluded","message":"Run ` + "`excluded command`" + `.","want":["excluded command"]}
	]`)
	calls := 0
	classifier := classifierFunc(func(_ context.Context, input autocopy.Input) (map[string]float64, error) {
		calls++
		if input.Message != "Run `selected command`." {
			t.Errorf("sent unselected case: %q", input.Message)
		}
		return map[string]float64{"c1": 1}, nil
	})
	report, err := Run(context.Background(), data, classifier, "test", "test", Options{CaseIDs: []string{"selected"}, CurrentOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(report.Results) != 1 || report.Metrics["before"] != nil || report.Metrics["after"].Exact != 1 {
		t.Fatalf("calls=%d report=%+v", calls, report)
	}
	for _, ids := range [][]string{{"selected", "unknown"}, {"selected", "selected"}, {""}} {
		_, err := Run(context.Background(), data, classifier, "test", "test", Options{CaseIDs: ids})
		if err == nil || calls != 1 {
			t.Fatalf("selection %q: err=%v calls=%d", ids, err, calls)
		}
	}
}
