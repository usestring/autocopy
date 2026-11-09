# Evaluation contract

`cases.json` is the reviewable source of labels. Each case has an identifier,
historical topic, kind, message, expected complete selections in source order,
and a note about the boundary it exercises. Products, addresses, destinations,
and operational identifiers were replaced before inclusion. No raw transcripts or
private source mapping are stored here. The examples use generic website and
watcher names; labels match the anonymized text rather than the original wording.

The corpus has 28 cases: 21 positive and 7 negative. Format and stress variants
share a historical source topic, so they are correlated examples, not independent
samples. The labels are manually chosen desired selections, not evidence of what
a user later copied.

## Two separate questions

1. **Can the extractor represent the wanted selection?** The default run supplies
   oracle scores derived from the labels and checks exact assembled text. This
   catches missing candidates, split drafts, duplicate links and boundary errors.
2. **Does the classifier select it?** `-live` sends the same messages and questions
   to the configured TypeSafe model. Its actual probabilities drive selection.
   A green oracle run says nothing about classifier precision or recall.

The frozen baseline in `internal/eval/baseline.go` preserves the earlier
candidate generator, copy question, 1,000-character block limit, 1,500-character
context and 0.8 threshold. It does not change when the current implementation is
tuned. The reported billing reply fits the old size limit; its unfenced variants
exercise candidate availability, and the longer stress case separately exercises
the old cutoff.

## Reproduce and inspect

```sh
go run ./cmd/autocopy-eval -cases evals/cases.json -out /tmp/autocopy-eval.json
```

Add `-live` only when the corpus is approved for transfer and the environment has
`TYPESAFE_API_KEY`. The default is offline even when the environment holds a key.
Importing either Go package does not read environment variables or start requests.

`results.json` contains the run mode, model, build revision when available,
threshold, current limits, corpus SHA-256, baseline/current summaries, and each
case's expected output, actual output, scores, input hash, error and elapsed time.

Exact-case accuracy requires every selection and its order to match. Correct,
missed and extra counts compare complete items; a partial reply is a missed
complete item plus an extra incorrect item. Precision and recall use those item
counts. Request errors are counted separately and always fail exact-case scoring;
an error cannot become a correct empty selection. The summary's latency is
available per case, not a benchmark of classifier speed on a controlled host.

The current corpus and score-to-output regressions run in CI with no live API.
Keep a failure as a separately labelled case before tuning. For larger future
runs, separate tuning cases from a held-out set and report results on both.

## Reading a failure

- Wanted draft absent from candidates: extraction recall failure, even with
  perfect classifier labels.
- Correct candidates, wrong probabilities: classifier selection failure.
- Correct paragraph labels, wrong joined output: assembly or boundary failure.
- No usable service response: request failure; do not count it as an abstention.

There is no committed live report yet. This repository makes the distinction
visible instead of presenting offline oracle scores as model-quality evidence.
