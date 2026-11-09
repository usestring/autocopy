package autocopy

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"
)

type classifierFunc func(context.Context, Input) (map[string]float64, error)

func (f classifierFunc) Classify(ctx context.Context, input Input) (map[string]float64, error) {
	return f(ctx, input)
}

func TestDraftAndEmbeddedLinkStayTogether(t *testing.T) {
	draft := "Hey, billing is at https://example.test/billing.\n\nYour unused balance carries over."
	msg := "Here's the revised reply.\n\n" + draft + "\n\nI checked the page; the changes below are proposals."
	got, err := Extract(context.Background(), msg, classifierFunc(func(_ context.Context, input Input) (map[string]float64, error) {
		scores := make(map[string]float64)
		for key := range input.Questions {
			scores[key] = 0
		}
		scores["p2"], scores["p3"], scores["c1"] = 1, 1, 1
		return scores, nil
	}))
	if err != nil || !slices.Equal(got, []string{draft}) {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestIncompleteOrInvalidScoresFailClosed(t *testing.T) {
	p := Prepare("Run `example command`.")
	for _, value := range []float64{-1, 2, math.NaN(), math.Inf(1)} {
		if _, err := p.Select(map[string]float64{"c1": value}); err == nil {
			t.Errorf("accepted %v", value)
		}
	}
	if _, err := p.Select(nil); err == nil {
		t.Fatal("accepted missing scores")
	}
}

func TestClassifierErrorsReturnNoCopy(t *testing.T) {
	failure := errors.New("classifier unavailable")
	got, err := Extract(context.Background(), "Run `example command`.", classifierFunc(func(context.Context, Input) (map[string]float64, error) { return nil, failure }))
	if got != nil || !errors.Is(err, failure) {
		t.Fatalf("got=%q err=%v", got, err)
	}
}
