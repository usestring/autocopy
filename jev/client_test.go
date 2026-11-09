package jev

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/usestring/autocopy"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestRequestAndAnswerContract(t *testing.T) {
	client := Client{Key: "fixture-value", HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer fixture-value" {
			t.Fatal("missing authentication")
		}
		var body struct {
			Model     string
			State     struct{ Message string }
			Questions map[string]struct{ Type, Instructions string }
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != Model || body.State.Message != "Run the command" || body.Questions["c1"].Type != "noul" || body.Questions["c1"].Instructions != "Copy this?" {
			t.Fatalf("invalid wire payload: %+v", body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"model":"` + Model + `","answers":{"c1":{"type":"noul","noul":0.9}}}`))}, nil
	})}}
	got, err := client.Classify(context.Background(), autocopy.Input{Message: "Run the command", Questions: map[string]string{"c1": "Copy this?"}})
	if err != nil || got["c1"] != 0.9 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestUnusableAnswersFail(t *testing.T) {
	for _, response := range []string{
		`{"model":"unexpected","answers":{"c1":{"type":"noul","noul":0.9}}}`,
		`{"model":"` + Model + `","answers":{}}`,
		`{"model":"` + Model + `","answers":{"c1":{"type":"noul","noul":null}}}`,
		`{"model":"` + Model + `","answers":{"c1":{"type":"noul","noul":2}}}`,
	} {
		c := Client{Key: "fixture-value", HTTPClient: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response))}, nil
		})}}
		if _, err := c.Classify(context.Background(), autocopy.Input{Questions: map[string]string{"c1": "Copy this?"}}); err == nil {
			t.Errorf("accepted %s", response)
		}
	}
}
