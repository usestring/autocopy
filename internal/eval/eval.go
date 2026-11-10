package eval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/usestring/autocopy"
)

type Case struct {
	ID      string   `json:"id"`
	Source  string   `json:"source"`
	Kind    string   `json:"kind"`
	Message string   `json:"message"`
	Want    []string `json:"want"`
	Notes   string   `json:"notes"`
}

type Result struct {
	ID          string             `json:"id"`
	Variant     string             `json:"variant"`
	Want        []string           `json:"want"`
	Got         []string           `json:"got"`
	Scores      map[string]float64 `json:"scores"`
	InputSHA256 string             `json:"input_sha256"`
	Error       string             `json:"error,omitempty"`
	Exact       bool               `json:"exact"`
	ElapsedMS   int64              `json:"elapsed_ms"`
}

type Metrics struct {
	Cases     int     `json:"cases"`
	Exact     int     `json:"exact"`
	Correct   int     `json:"correct_items"`
	Missed    int     `json:"missed_items"`
	Extra     int     `json:"extra_items"`
	Errors    int     `json:"request_errors"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
}

type Report struct {
	SchemaVersion          int                 `json:"schema_version"`
	Mode                   string              `json:"mode"`
	Model                  string              `json:"model"`
	Revision               string              `json:"revision"`
	CorpusSHA256           string              `json:"corpus_sha256"`
	Threshold              float64             `json:"threshold"`
	CommandThreshold       float64             `json:"command_threshold"`
	ProsePresenceThreshold float64             `json:"prose_presence_threshold"`
	ProseMargin            float64             `json:"prose_margin"`
	CandidateMaxChars      int                 `json:"candidate_max_chars"`
	ContextMaxChars        int                 `json:"context_max_chars"`
	Metrics                map[string]*Metrics `json:"metrics"`
	Results                []Result            `json:"results"`
}

func Digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func Oracle(input autocopy.Input, want []string) map[string]float64 {
	scores := make(map[string]float64, len(input.Questions))
	for name, question := range input.Questions {
		scores[name] = 0
		if name == "has_draft" {
			if len(want) > 0 {
				scores[name] = 1
			}
			continue
		}
		commentary := strings.HasPrefix(name, "mp")
		paragraph := strings.HasPrefix(name, "p") || commentary
		if commentary {
			scores[name] = 1
		}
		marker := "Text: "
		if paragraph && !commentary {
			marker = "Paragraph: "
		}
		_, text, ok := strings.Cut(question, marker)
		if !ok {
			continue
		}
		for _, expected := range want {
			if text == expected || paragraph && strings.Contains(expected, text) {
				scores[name] = 1
				if commentary {
					scores[name] = 0
				}
			}
		}
	}
	return scores
}

type Options struct {
	CaseIDs     []string
	CurrentOnly bool
}

func Run(ctx context.Context, data []byte, classifier autocopy.Classifier, model, revision string, options Options) (*Report, error) {
	var cases []Case
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("empty corpus")
	}
	ids := make(map[string]bool)
	for _, c := range cases {
		if c.ID == "" || ids[c.ID] {
			return nil, fmt.Errorf("missing or duplicate case identifier")
		}
		ids[c.ID] = true
		for _, want := range c.Want {
			if want == "" || !strings.Contains(c.Message, want) {
				return nil, fmt.Errorf("%s: expected output is not intact source text", c.ID)
			}
		}
	}
	if len(options.CaseIDs) > 0 {
		selected := make(map[string]bool)
		for _, id := range options.CaseIDs {
			if !ids[id] || selected[id] {
				return nil, fmt.Errorf("unknown or duplicate selected case %q", id)
			}
			selected[id] = true
		}
		cases = slices.DeleteFunc(cases, func(c Case) bool { return !selected[c.ID] })
	}
	mode := "oracle"
	if classifier != nil {
		mode = "live"
	} else {
		model = "oracle-labels"
	}
	variants := []string{"before", "after"}
	if options.CurrentOnly {
		variants = []string{"after"}
	}
	metrics := make(map[string]*Metrics, len(variants))
	for _, variant := range variants {
		metrics[variant] = &Metrics{}
	}
	report := &Report{SchemaVersion: 1, Mode: mode, Model: model, Revision: revision, CorpusSHA256: Digest(data), Threshold: autocopy.Threshold,
		CommandThreshold:       autocopy.CommandThreshold,
		ProsePresenceThreshold: autocopy.ProsePresenceThreshold,
		ProseMargin:            autocopy.ProseMargin,
		CandidateMaxChars:      autocopy.CandidateMaxChars, ContextMaxChars: autocopy.ContextMaxChars,
		Metrics: metrics,
	}
	for _, c := range cases {
		for _, variant := range variants {
			plan := autocopy.Prepare(c.Message)
			input := plan.Input()
			var candidates []string
			if variant == "before" {
				input, candidates = baseline(c.Message)
			}
			encoded, err := json.Marshal(input)
			if err != nil {
				return nil, err
			}
			row := Result{ID: c.ID, Variant: variant, Want: c.Want, InputSHA256: Digest(encoded)}
			if classifier == nil {
				row.Scores = Oracle(input, c.Want)
			} else if len(input.Questions) > 0 {
				started := time.Now()
				callCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
				row.Scores, err = classifier.Classify(callCtx, input)
				cancel()
				row.ElapsedMS = time.Since(started).Milliseconds()
			}
			if err == nil {
				if variant == "after" {
					row.Got, err = plan.Select(row.Scores)
				} else {
					for i, candidate := range candidates {
						score, ok := row.Scores[fmt.Sprintf("c%d", i+1)]
						if !ok || math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 1 {
							err = fmt.Errorf("invalid or missing baseline probability")
							break
						}
						if score >= 0.8 {
							row.Got = append(row.Got, candidate)
						}
					}
				}
			}
			if err != nil {
				row.Error = err.Error()
				row.Got = nil
			}
			row.Exact = err == nil && slices.Equal(row.Got, row.Want)
			report.Results = append(report.Results, row)
			m := report.Metrics[variant]
			m.Cases++
			if row.Exact {
				m.Exact++
			}
			if row.Error != "" {
				m.Errors++
			}
			for _, want := range row.Want {
				if slices.Contains(row.Got, want) {
					m.Correct++
				} else {
					m.Missed++
				}
			}
			for _, got := range row.Got {
				if !slices.Contains(row.Want, got) {
					m.Extra++
				}
			}
		}
	}
	for _, m := range report.Metrics {
		if n := m.Correct + m.Extra; n > 0 {
			m.Precision = float64(m.Correct) / float64(n)
		}
		if n := m.Correct + m.Missed; n > 0 {
			m.Recall = float64(m.Correct) / float64(n)
		}
	}
	return report, nil
}
