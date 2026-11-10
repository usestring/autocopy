// Package autocopy extracts copyable commands, links, and drafts from assistant
// messages. Classification is explicit and supplied by the caller.
package autocopy

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	jevCopyThreshold    = 0.80
	jevCopyCandidates   = 8
	jevCopyCandidateMax = 12000
	jevCopyMinChars     = 4
	jevCopyMessageChars = 24000
)

// jevCopyQuestion is asked once per candidate, with the candidate quoted.
const jevCopyQuestion = "Is this text a command supplied for the user to run? Text: "

const jevCopyCandidateDraftQuestion = "Is this text the reply or document draft presented by the assistant? Text: "

const jevCopyValueQuestion = "Is this text a link or value supplied for the user to open or paste? Text: "

const jevCopyDraftQuestion = "This paragraph is part of the message or document the assistant is drafting " +
	"for the user to send or paste elsewhere. Paragraph: "

const jevHasDraftQuestion = "Does the assistant provide an unsent message or document draft for the user to copy elsewhere?"

var (
	copyParagraphBreakPattern    = regexp.MustCompile(`\r?\n[ \t]*\r?\n`)
	copyFenceLinePattern         = regexp.MustCompile("(?m)^ {0,3}(`{3,})([^\\r\\n]*)\\r?$")
	copySpanPattern              = regexp.MustCompile("`([^`\\n]+)`")
	copyURLPattern               = regexp.MustCompile(`https?://[^\s<>\[\]"'` + "`" + `]+`)
	copyProseContinuationPattern = regexp.MustCompile(`^[ \t]+\p{L}`)
	copyCandidatePattern         = regexp.MustCompile(copySpanPattern.String() + "|(" + copyURLPattern.String() + ")")
)

type copyFence struct {
	start, end               int
	contentStart, contentEnd int
}

func copyFences(message string) []copyFence {
	var fences []copyFence
	var open *copyFence
	var width int
	for _, line := range copyFenceLinePattern.FindAllStringSubmatchIndex(message, -1) {
		markerWidth := line[3] - line[2]
		tail := message[line[4]:line[5]]
		if open == nil {
			if strings.Contains(tail, "`") || line[1] == len(message) {
				continue
			}
			open = &copyFence{start: line[0], contentStart: line[1] + 1}
			width = markerWidth
		} else if markerWidth >= width && strings.TrimSpace(tail) == "" {
			open.end = line[1]
			open.contentEnd = line[0]
			fences = append(fences, *open)
			open = nil
		}
	}
	return fences
}

type copyCandidate struct {
	start int
	text  string
}

// Source offsets distinguish copyable spans from earlier mentions of the same text.
// Text too short to be worth a key, or too long to send, is left out
// rather than cut, since a truncated command is worse than none.
func copyCandidates(message string) []copyCandidate {
	var out []copyCandidate
	seen := map[string]bool{}
	add := func(raw string, start int) {
		text := strings.TrimSpace(raw)
		start += strings.Index(raw, text)
		n := utf8.RuneCountInString(text)
		if n < jevCopyMinChars || n > jevCopyCandidateMax || seen[text] || len(out) >= jevCopyCandidates {
			return
		}
		seen[text] = true
		out = append(out, copyCandidate{start: start, text: text})
	}
	addInline := func(start, end int) {
		for _, match := range copyCandidatePattern.FindAllStringSubmatchIndex(message[start:end], -1) {
			if match[2] >= 0 {
				add(message[start+match[2]:start+match[3]], start+match[2])
				continue
			}
			left, right := start+match[4], start+match[5]
			link := message[left:right]
			if (left == 0 || message[left-1] != '<' && message[left-1] != '(') &&
				copyProseContinuationPattern.MatchString(message[right:]) {
				link = strings.TrimRight(link, ".,;:!?")
			}
			depth := 0
			for i, char := range link {
				if char == '(' {
					depth++
				} else if char == ')' {
					if depth == 0 {
						link = link[:i]
						break
					}
					depth--
				}
			}
			add(link, left)
		}
	}
	start := 0
	for _, fence := range copyFences(message) {
		addInline(start, fence.start)
		add(message[fence.contentStart:fence.contentEnd], fence.contentStart)
		start = fence.end
	}
	addInline(start, len(message))
	return out
}

type copyProsePart struct {
	start int
	end   int
}

func copyDraftParts(message string) []copyProsePart {
	// A complete context avoids treating a caveat beyond the limit as draft text.
	if utf8.RuneCountInString(message) > jevCopyMessageChars {
		return nil
	}
	fences := copyFences(message)
	breaks := copyParagraphBreakPattern.FindAllStringIndex(message, -1)
	if len(breaks) == 0 {
		return nil
	}
	var parts []copyProsePart
	start := 0
	for _, boundary := range append(breaks, []int{len(message), len(message)}) {
		end := boundary[0]
		text := strings.TrimSpace(message[start:end])
		left := start + strings.Index(message[start:end], text)
		part := copyProsePart{start: left, end: left + len(text)}
		inFence := false
		for _, fence := range fences {
			if part.start < fence.end && part.end > fence.start {
				inFence = true
				break
			}
		}
		n := utf8.RuneCountInString(text)
		if !inFence && n >= jevCopyMinChars && n <= jevCopyCandidateMax {
			parts = append(parts, part)
		}
		start = boundary[1]
	}
	if len(parts) > 32 {
		return nil
	}
	return parts
}

func jevCopyQuestions(message string, candidates []copyCandidate) map[string]string {
	questions := make(map[string]string, 3*len(candidates))
	for i, candidate := range candidates {
		questions[fmt.Sprintf("c%d", i+1)] = jevCopyQuestion + candidate.text
		questions[fmt.Sprintf("d%d", i+1)] = jevCopyCandidateDraftQuestion + candidate.text
		questions[fmt.Sprintf("v%d", i+1)] = jevCopyValueQuestion + candidate.text
	}
	parts := copyDraftParts(message)
	if len(parts) > 0 || len(candidates) > 0 {
		questions["has_draft"] = jevHasDraftQuestion
	}
	for i, part := range parts {
		questions[fmt.Sprintf("p%d", i+1)] = jevCopyDraftQuestion + message[part.start:part.end]
	}
	return questions
}

func pickJevCopy(message string, candidates []copyCandidate, answers map[string]float64) []string {
	var drafts []copyProsePart
	for i, part := range copyDraftParts(message) {
		// A status paragraph can sound copyable even when no draft exists.
		if answers["has_draft"] < jevCopyThreshold || answers[fmt.Sprintf("p%d", i+1)] < jevCopyThreshold {
			continue
		}
		if len(drafts) > 0 && strings.TrimSpace(message[drafts[len(drafts)-1].end:part.start]) == "" {
			drafts[len(drafts)-1].end = part.end
		} else {
			drafts = append(drafts, part)
		}
	}
	type pick struct {
		start int
		text  string
	}
	drafts = slices.DeleteFunc(drafts, func(draft copyProsePart) bool {
		return utf8.RuneCountInString(message[draft.start:draft.end]) > jevCopyCandidateMax
	})
	var picks []pick
	for _, draft := range drafts {
		picks = append(picks, pick{draft.start, message[draft.start:draft.end]})
	}
	for i, candidate := range candidates {
		command := answers[fmt.Sprintf("c%d", i+1)] >= jevCopyThreshold
		draft := answers["has_draft"] >= jevCopyThreshold && answers[fmt.Sprintf("d%d", i+1)] >= jevCopyThreshold
		value := answers[fmt.Sprintf("v%d", i+1)] >= jevCopyThreshold
		if !command && !draft && !value {
			continue
		}
		start := candidate.start
		contained := false
		for _, draft := range drafts {
			if start >= draft.start && start+len(candidate.text) <= draft.end {
				contained = true
				break
			}
		}
		if !contained {
			picks = append(picks, pick{start, candidate.text})
		}
	}
	slices.SortStableFunc(picks, func(a, b pick) int { return cmp.Compare(a.start, b.start) })
	var out []string
	for _, pick := range picks {
		out = append(out, pick.text)
	}
	return out
}

const (
	Threshold         = jevCopyThreshold
	CandidateMaxChars = jevCopyCandidateMax
	ContextMaxChars   = jevCopyMessageChars
)

type Input struct {
	Message   string            `json:"message"`
	Questions map[string]string `json:"questions"`
}

type Classifier interface {
	Classify(context.Context, Input) (map[string]float64, error)
}

type Plan struct {
	message    string
	candidates []copyCandidate
	questions  map[string]string
}

func Prepare(message string) *Plan {
	candidates := copyCandidates(message)
	return &Plan{message: message, candidates: candidates, questions: jevCopyQuestions(message, candidates)}
}

func (p *Plan) Input() Input {
	return Input{Message: boundedText(p.message, ContextMaxChars), Questions: maps.Clone(p.questions)}
}

func (p *Plan) Select(scores map[string]float64) ([]string, error) {
	for name := range p.questions {
		value, ok := scores[name]
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			return nil, fmt.Errorf("invalid or missing probability for %s", name)
		}
	}
	return pickJevCopy(p.message, p.candidates, scores), nil
}

func Extract(ctx context.Context, message string, classifier Classifier) ([]string, error) {
	plan := Prepare(message)
	input := plan.Input()
	if len(input.Questions) == 0 {
		return nil, nil
	}
	if classifier == nil {
		return nil, errors.New("autocopy: a classifier is required")
	}
	scores, err := classifier.Classify(ctx, input)
	if err != nil {
		return nil, err
	}
	return plan.Select(scores)
}

func boundedText(text string, maxChars int) string {
	if utf8.RuneCountInString(text) <= maxChars {
		return text
	}
	return string([]rune(text)[:maxChars])
}
