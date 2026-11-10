package autocopy

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
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
		scores["has_draft"], scores["p2"], scores["p3"], scores["c1"] = 1, 1, 1, 1
		return scores, nil
	}))
	if err != nil || !slices.Equal(got, []string{draft}) {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestIncompleteOrInvalidScoresFailClosed(t *testing.T) {
	p := Prepare("Run `example command`.")
	for _, value := range []float64{-1, 2, math.NaN(), math.Inf(1)} {
		if _, err := p.Select(map[string]float64{"c1": value, "d1": 0, "v1": 0, "has_draft": 0}); err == nil {
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

func TestDraftPresenceDoesNotSuppressCommand(t *testing.T) {
	plan := Prepare("Run `example command`.\n\nAll tests pass. The change is ready for review.")
	scores := map[string]float64{"c1": 1, "d1": 0, "v1": 0, "p1": 0.9, "p2": 0.9, "has_draft": 0.1}
	got, err := plan.Select(scores)
	if err != nil || !slices.Equal(got, []string{"example command"}) {
		t.Fatalf("got=%q err=%v", got, err)
	}
	delete(scores, "has_draft")
	if _, err := plan.Select(scores); err == nil {
		t.Fatal("accepted a missing draft-presence answer")
	}
}

func TestSelectionsUseExtractedCandidatePositions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		message string
		draft   bool
		want    []string
	}{
		{
			name:    "order",
			message: "The phrase example command is a label. Run `other command`, then `example command`.",
			want:    []string{"other command", "example command"},
		},
		{
			name:    "containment",
			message: "The phrase example command is a label.\n\nRun `other command`, then `example command`.",
			draft:   true,
			want:    []string{"The phrase example command is a label.", "other command", "example command"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := Prepare(tc.message)
			scores := make(map[string]float64)
			for key := range plan.Input().Questions {
				scores[key] = 0
			}
			scores["c1"], scores["c2"] = 1, 1
			if tc.draft {
				scores["has_draft"], scores["p1"] = 1, 1
			}
			got, err := plan.Select(scores)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("got=%q want=%q err=%v", got, tc.want, err)
			}
		})
	}
}

func TestDiscardedDraftDoesNotSuppressCommand(t *testing.T) {
	message := strings.Repeat("a", 6500) + " `example command`\n\n" + strings.Repeat("b", 6500)
	plan := Prepare(message)
	got, err := plan.Select(map[string]float64{"c1": 1, "d1": 0, "v1": 0, "has_draft": 1, "p1": 1, "p2": 1})
	if err != nil || !slices.Equal(got, []string{"example command"}) {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestCandidateIntentsSelectIntactText(t *testing.T) {
	for _, tc := range []struct{ name, message, score, want string }{
		{"draft", "Here's the reply:\n\n```\nHey — thanks for the update.\n\nI will follow up tomorrow.\n```", "d1", "Hey — thanks for the update.\n\nI will follow up tomorrow."},
		{"link", "Open <https://example.test/runbook>.", "v1", "https://example.test/runbook"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := Prepare(tc.message)
			scores := make(map[string]float64)
			for key := range plan.Input().Questions {
				scores[key] = 0
			}
			scores[tc.score] = 0.95
			if tc.name == "draft" {
				scores["has_draft"] = 1
			}
			got, err := plan.Select(scores)
			if err != nil || !slices.Equal(got, []string{tc.want}) {
				t.Fatalf("got=%q err=%v", got, err)
			}
		})
	}
}

func TestSentDraftDoesNotBecomeCopyCandidate(t *testing.T) {
	plan := Prepare("I sent this reply:\n\n```\nThanks for the update.\n```\n\nDelivery confirmed.")
	scores := make(map[string]float64)
	for key := range plan.Input().Questions {
		scores[key] = 0
	}
	scores["d1"] = 0.95
	got, err := plan.Select(scores)
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%q err=%v", got, err)
	}
}
