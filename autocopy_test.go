package autocopy

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestCopyCandidatesTakesBlocksSpansAndLinksInOrder(t *testing.T) {
	message := "Run this yourself:\n\n```bash\ngcloud auth login\n```\n\n" +
		"Then type `! bun run deploy` and open <https://console.cloud.google.com/run>, then check `x`.\n" +
		"The link `https://example.com/inside` is quoted once."
	got := copyCandidateTexts(message)
	want := []string{"gcloud auth login", "! bun run deploy", "https://console.cloud.google.com/run", "https://example.com/inside"}
	if !slices.Equal(got, want) {
		t.Fatalf("candidates = %q, want %q", got, want)
	}
}

func TestCopyCandidatesPreservesMixedOrderBeforeLimit(t *testing.T) {
	message := "Open https://example.com/first then run `first-command`.\n"
	want := []string{"https://example.com/first", "first-command"}
	for i := range jevCopyCandidates {
		command := fmt.Sprintf("command-%d", i)
		message += "```sh\n" + command + "\n```\n"
		if len(want) < jevCopyCandidates {
			want = append(want, command)
		}
	}
	if got := copyCandidateTexts(message); !slices.Equal(got, want) {
		t.Fatalf("candidates = %q, want %q", got, want)
	}
}

func TestCopyCandidatesPreservesURLParentheses(t *testing.T) {
	for _, message := range []string{
		"https://en.wikipedia.org/wiki/Foo_(bar)",
		"[link](https://en.wikipedia.org/wiki/Foo_(bar)).",
		"(https://en.wikipedia.org/wiki/Foo_(bar)),",
		"https://en.wikipedia.org/wiki/Foo_(bar)).after",
	} {
		if got := copyCandidateTexts(message); !slices.Equal(got, []string{"https://en.wikipedia.org/wiki/Foo_(bar)"}) {
			t.Errorf("message %q: candidates = %q", message, got)
		}
	}
	message := "[nested](https://example.com/a_(b_(c))) and <https://example.com/plain>."
	want := []string{"https://example.com/a_(b_(c))", "https://example.com/plain"}
	if got := copyCandidateTexts(message); !slices.Equal(got, want) {
		t.Fatalf("candidates = %q, want %q", got, want)
	}
}

func TestCopyCandidatesPreservesURLTrailingPunctuation(t *testing.T) {
	for _, suffix := range []string{"!", "?", ".", ",", ";", ":", "!!", "?q=value!"} {
		link := "https://example.test/run" + suffix
		for _, message := range []string{link, "[run](" + link + ").", "<" + link + ">,", "`" + link + "`"} {
			if got := copyCandidateTexts(message); !slices.Equal(got, []string{link}) {
				t.Errorf("message %q: candidates = %q, want %q", message, got, link)
			}
		}
	}
}

func TestCopyCandidatesStripsPunctuationBeforeContinuingProse(t *testing.T) {
	for _, punctuation := range []string{",", ".", ";", ":", "!", "?"} {
		link := "https://example.test/run"
		for _, message := range []string{"Open " + link + punctuation + " then continue", link + punctuation + "\tNext step"} {
			if got := copyCandidateTexts(message); !slices.Equal(got, []string{link}) {
				t.Errorf("message %q: candidates = %q, want %q", message, got, link)
			}
		}
		for _, message := range []string{"<" + link + punctuation + "> then continue", "[run](" + link + punctuation + ") then continue", "`" + link + punctuation + "` then continue"} {
			if got := copyCandidateTexts(message); !slices.Equal(got, []string{link + punctuation}) {
				t.Errorf("message %q: candidates = %q, want %q", message, got, link+punctuation)
			}
		}
	}
}

func TestCopyCandidatesLeavesOutWhatIsTooLongRatherThanCutting(t *testing.T) {
	long := strings.Repeat("a", jevCopyCandidateMax+1)
	if got := copyCandidateTexts("`" + long + "` and `ok-value`"); !slices.Equal(got, []string{"ok-value"}) {
		t.Fatalf("candidates = %q", got)
	}
}

func copyCandidateTexts(message string) []string {
	var texts []string
	for _, candidate := range copyCandidates(message) {
		texts = append(texts, candidate.text)
	}
	return texts
}

func TestFencedCommandsPreserveLiteralBackticks(t *testing.T) {
	for _, tc := range []struct{ name, fence, body string }{
		{"literal", "```", "printf '%s\\n' '```'\necho done"},
		{"longer fence", "````", "echo before\n```\necho after"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := "Run this command:\n\n" + tc.fence + "sh\n" + tc.body + "\n" + tc.fence + "\n\nThen run `next command`."
			if got := copyCandidateTexts(message); !slices.Equal(got, []string{tc.body, "next command"}) {
				t.Fatalf("candidates=%q", got)
			}
			plan := Prepare(message)
			scores := make(map[string]float64)
			for key := range plan.Input().Questions {
				scores[key] = 0
			}
			scores["c1"], scores["c2"] = 1, 1
			got, err := plan.Select(scores)
			if err != nil || !slices.Equal(got, []string{tc.body, "next command"}) {
				t.Fatalf("got=%q err=%v", got, err)
			}
		})
	}
}
