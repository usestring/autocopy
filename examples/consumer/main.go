package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/usestring/autocopy"
)

// commandClassifier answers yes only for command questions about candidates.
type commandClassifier struct{}

func (commandClassifier) Classify(_ context.Context, in autocopy.Input) (map[string]float64, error) {
	scores := map[string]float64{}
	for key, question := range in.Questions {
		if strings.HasPrefix(question, "Is this text a command") {
			scores[key] = 1
		} else {
			scores[key] = 0
		}
	}
	return scores, nil
}

func main() {
	picks, err := autocopy.Extract(context.Background(), "Run `go test ./...` before you push.", commandClassifier{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%q\n", picks)
}
