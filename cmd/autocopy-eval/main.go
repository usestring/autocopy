package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/usestring/autocopy"
	"github.com/usestring/autocopy/internal/eval"
	"github.com/usestring/autocopy/jev"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cases := flag.String("cases", "evals/cases.json", "labelled corpus")
	output := flag.String("out", "", "write machine-readable results to this file")
	live := flag.Bool("live", false, "send fixture messages to TypeSafe using TYPESAFE_API_KEY")
	only := flag.String("only", "", "comma-separated case IDs to evaluate")
	currentOnly := flag.Bool("current-only", false, "skip the frozen baseline")
	flag.Parse()
	options := eval.Options{CurrentOnly: *currentOnly}
	if *only != "" {
		for _, id := range strings.Split(*only, ",") {
			options.CaseIDs = append(options.CaseIDs, strings.TrimSpace(id))
		}
	}
	data, err := os.ReadFile(*cases)
	if err != nil {
		return err
	}
	var classifier autocopy.Classifier
	if *live {
		key := os.Getenv("TYPESAFE_API_KEY")
		if key == "" {
			return fmt.Errorf("live eval requires TYPESAFE_API_KEY")
		}
		classifier = &jev.Client{Key: key}
	}
	revision := "development"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				revision = setting.Value
			}
		}
	}
	report, err := eval.Run(context.Background(), data, classifier, jev.Model, revision, options)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if *output != "" {
		if err := os.WriteFile(*output, append(encoded, '\n'), 0o600); err != nil {
			return err
		}
	}
	fmt.Printf("mode=%s corpus_sha256=%s\n", report.Mode, report.CorpusSHA256)
	if report.Mode == "oracle" {
		fmt.Println("Oracle labels test extraction and assembly, not model accuracy.")
	}
	for _, variant := range []string{"before", "after"} {
		m := report.Metrics[variant]
		if m == nil {
			continue
		}
		fmt.Printf("%s: exact=%d/%d correct=%d missed=%d extra=%d request_errors=%d precision=%.3f recall=%.3f\n", variant, m.Exact, m.Cases, m.Correct, m.Missed, m.Extra, m.Errors, m.Precision, m.Recall)
	}
	if m := report.Metrics["after"]; m.Exact != m.Cases {
		return fmt.Errorf("%d current cases failed", m.Cases-m.Exact)
	}
	return nil
}
