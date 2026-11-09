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
	got := copyCandidates(message)
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
	if got := copyCandidates(message); !slices.Equal(got, want) {
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
		if got := copyCandidates(message); !slices.Equal(got, []string{"https://en.wikipedia.org/wiki/Foo_(bar)"}) {
			t.Errorf("message %q: candidates = %q", message, got)
		}
	}
	message := "[nested](https://example.com/a_(b_(c))) and <https://example.com/plain>."
	want := []string{"https://example.com/a_(b_(c))", "https://example.com/plain"}
	if got := copyCandidates(message); !slices.Equal(got, want) {
		t.Fatalf("candidates = %q, want %q", got, want)
	}
}

func TestCopyCandidatesPreservesURLTrailingPunctuation(t *testing.T) {
	for _, suffix := range []string{"!", "?", ".", ",", ";", ":", "!!", "?q=value!"} {
		link := "https://example.test/run" + suffix
		for _, message := range []string{link, "[run](" + link + ").", "<" + link + ">,", "`" + link + "`"} {
			if got := copyCandidates(message); !slices.Equal(got, []string{link}) {
				t.Errorf("message %q: candidates = %q, want %q", message, got, link)
			}
		}
	}
}

func TestCopyCandidatesStripsPunctuationBeforeContinuingProse(t *testing.T) {
	for _, punctuation := range []string{",", ".", ";", ":", "!", "?"} {
		link := "https://example.test/run"
		for _, message := range []string{"Open " + link + punctuation + " then continue", link + punctuation + "\tNext step"} {
			if got := copyCandidates(message); !slices.Equal(got, []string{link}) {
				t.Errorf("message %q: candidates = %q, want %q", message, got, link)
			}
		}
		for _, message := range []string{"<" + link + punctuation + "> then continue", "[run](" + link + punctuation + ") then continue", "`" + link + punctuation + "` then continue"} {
			if got := copyCandidates(message); !slices.Equal(got, []string{link + punctuation}) {
				t.Errorf("message %q: candidates = %q, want %q", message, got, link+punctuation)
			}
		}
	}
}

func TestCopyCandidatesLeavesOutWhatIsTooLongRatherThanCutting(t *testing.T) {
	long := strings.Repeat("a", jevCopyCandidateMax+1)
	if got := copyCandidates("`" + long + "` and `ok-value`"); !slices.Equal(got, []string{"ok-value"}) {
		t.Fatalf("candidates = %q", got)
	}
}
