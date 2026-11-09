package eval

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/usestring/autocopy"
)

// Frozen pre-fix behavior keeps future comparisons from moving the baseline.
const (
	jevCopyThreshold    = 0.80
	jevCopyCandidates   = 8
	jevCopyCandidateMax = 1000
	jevCopyMinChars     = 4
	jevCopyMessageChars = 1500
)

// jevCopyQuestion is asked once per candidate, with the candidate quoted.
const jevCopyQuestion = "Does the latest assistant message ask the user to copy this exact text and use it themselves: " +
	"run it in their own terminal, open it in a browser, or paste it into a form, chat or another app? " +
	"Answer no for text that only explains or names something, and for commands the agent runs itself. Text: "

var (
	copyFencePattern             = regexp.MustCompile("(?s)```[^\\n`]*\\n(.*?)```")
	copySpanPattern              = regexp.MustCompile("`([^`\\n]+)`")
	copyURLPattern               = regexp.MustCompile(`https?://[^\s<>\[\]"'` + "`" + `]+`)
	copyProseContinuationPattern = regexp.MustCompile(`^[ \t]+\p{L}`)
	copyCandidatePattern         = regexp.MustCompile(copyFencePattern.String() + "|" + copySpanPattern.String() + "|(" + copyURLPattern.String() + ")")
)

// copyCandidates is the text in message an operator might lift, in the
// order it appears: fenced blocks, code spans and links outside them.
// Text too short to be worth a key, or too long to send, is left out
// rather than cut, since a truncated command is worse than none.
func copyCandidates(message string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(text string) {
		text = strings.TrimSpace(text)
		n := utf8.RuneCountInString(text)
		if n < jevCopyMinChars || n > jevCopyCandidateMax || seen[text] || len(out) >= jevCopyCandidates {
			return
		}
		seen[text] = true
		out = append(out, text)
	}
	for _, match := range copyCandidatePattern.FindAllStringSubmatchIndex(message, -1) {
		switch {
		case match[2] >= 0:
			add(message[match[2]:match[3]])
		case match[4] >= 0:
			add(message[match[4]:match[5]])
		case match[6] >= 0:
			start, end := match[6], match[7]
			link := message[start:end]
			if (start == 0 || message[start-1] != '<' && message[start-1] != '(') &&
				copyProseContinuationPattern.MatchString(message[end:]) {
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
			add(link)
		}
	}
	return out
}

func baseline(message string) (autocopy.Input, []string) {
	candidates := copyCandidates(message)
	questions := make(map[string]string)
	for i, candidate := range candidates {
		questions[fmt.Sprintf("c%d", i+1)] = jevCopyQuestion + candidate
	}
	if utf8.RuneCountInString(message) > jevCopyMessageChars {
		message = string([]rune(message)[:jevCopyMessageChars])
	}
	return autocopy.Input{Message: message, Questions: questions}, candidates
}
