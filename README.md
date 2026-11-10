# autocopy

Extract the commands, links, and ready-to-send drafts a user might copy from an
assistant reply. This Go package is independent of any terminal UI or clipboard.

## Show the result, then reproduce it

Given this assistant reply:

```text
Here's the revised reply.

Hey, billing is at https://example.test/billing.

Your unused balance carries over.

I checked the page; the changes below are proposals.
```

The intended selection is one complete draft:

```text
Hey, billing is at https://example.test/billing.

Your unused balance carries over.
```

The introduction and commentary stay out. The link stays inside the draft instead
of becoming an extra selection. Whether a classifier actually chooses those
paragraphs is a separate question from whether the extractor can assemble them.

Start with the evals:

```sh
go test ./...
go run ./cmd/autocopy-eval -out /tmp/autocopy-eval.json
```

No key or network request is needed. The committed corpus has 28 manually labelled,
anonymized cases adapted from historical assistant replies: 21 positive and 7
negative cases, including implicit drafts, commands, explanatory snippets,
already-sent messages, Unicode, and size limits.

| Offline oracle run | Exact cases | Missed items | Extra items |
| --- | ---: | ---: | ---: |
| Frozen earlier extractor | 24/28 | 4 | 0 |
| Current extractor | 28/28 | 0 | 0 |

**These are extraction and assembly results with oracle labels, not model
accuracy.** The unfenced draft variants and longer draft cannot be recovered by
the earlier candidate generator even with perfect labels. The oracle deliberately
supplies those labels so the deterministic part can be checked independently.

There is no published live classifier benchmark. Do not read the table as a
claim that the package finds every draft correctly. This is a small, selected
regression set, not a held-out or representative benchmark.

[Read the eval contract and failure rules](evals/README.md).

## Import it

Requires Go 1.24 or newer. The first release is in preparation; until it is tagged,
there is no stable version to pin.

```go
import (
    "context"
    "os"

    "github.com/usestring/autocopy"
    "github.com/usestring/autocopy/jev"
)

// This explicit call sends the reply to TypeSafe. Importing the package does not.
picks, err := autocopy.Extract(context.Background(), reply, &jev.Client{
    Key: os.Getenv("TYPESAFE_API_KEY"),
})
```

Handle `err` before using the returned strings. `picks` follow source order and
preserve source text. Your application controls the clipboard and any later use.
Returned commands are text, never executed by this package.

The core has no third-party dependencies and accepts any implementation of:

```go
type Classifier interface {
    Classify(context.Context, autocopy.Input) (map[string]float64, error)
}
```

`autocopy.Prepare(reply)` exposes the bounded message and classification questions
through `Input()`. `Select(probabilities)` validates the answers and assembles the
picks. Use that pair to supply a different classifier or evaluate recorded scores.
An absent, invalid, or failed answer produces an error rather than a partial copy.

## Test the classifier explicitly

The live command sends the fixture messages to the TypeSafe JEV endpoint and uses
the key from the environment. Run it only with permission to send that corpus:

```sh
export TYPESAFE_API_KEY='your-key'
go run ./cmd/autocopy-eval -live -current-only \
  -only terraform-plan,open-runbook,billing-initial-unfenced,status-only \
  -out /tmp/autocopy-small.json
```

Start with a few cases during prompt development. This command evaluates only
four selected cases and skips the frozen baseline, making at most one request per
case. Unknown or duplicate identifiers fail before any request. These are tuning
cases; getting them right does not establish accuracy on unseen messages.

Omit `-only` to evaluate the full corpus, and omit `-current-only` to compare the
frozen baseline. That comparison makes at most two requests per case.
The report includes exact expected and actual selections, scores, request errors,
latency, model, threshold, corpus hash, and an input hash per case. A failed request
is a failed case, including when the
expected selection is empty. The command writes the report before failing if any
current case differs from its label. Live runs are excluded from CI.

## Current boundaries

The selector considers at most eight fenced blocks, inline code spans, or links,
and at most 32 prose paragraphs. It keeps intact copyable text up to 12,000 Unicode
characters, with up to 24,000 characters of message context. Overlong blocks are
omitted rather than truncated. Prose draft selection requires paragraph boundaries
and complete message context; it is omitted when the whole message exceeds that
context limit. Classification uses a confidence threshold of 0.8. Prose selection
requires both a positive draft-presence answer and positive paragraph answers;
command and link selections do not depend on draft presence.

The model can still select the wrong text or miss a paragraph. The committed
cases make those mistakes visible when they occur; they do not replace checking
content before using it.

## Origin and license

Extracted from [Gate Inbox](https://github.com/usestring/gate-inbox). The integration
back into Gate Inbox is a follow-up to the first package release.

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
