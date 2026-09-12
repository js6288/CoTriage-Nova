package infer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestHTTPClassifierPredict(t *testing.T) {
	classifier, err := NewHTTPClassifier("http://classifier.local/predict", time.Second)
	if err != nil {
		t.Fatalf("NewHTTPClassifier() error = %v", err)
	}
	classifier.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		var input ClassifierInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if input.TemplateText == "" || input.TemplateID == "" {
			t.Fatalf("input = %+v, want template fields", input)
		}
		var body bytes.Buffer
		_ = json.NewEncoder(&body).Encode(ClassifierResult{
			Label: "ALERT", LabelID: 1, AlertProbability: 0.451605, Threshold: 0.25,
			Probabilities: map[string]float64{"NON_ALERT": 0.548395, "ALERT": 0.451605},
			ModelVersion:  "bgl-distilbert",
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(&body),
		}, nil
	})

	result, err := classifier.Predict(context.Background(), ClassifierInput{TemplateID: "t1", TemplateText: "KERNEL FATAL rts panic"})
	if err != nil {
		t.Fatalf("Predict() error = %v", err)
	}
	if result.Label != "ALERT" || result.AlertProbability != 0.451605 || result.ModelVersion != "bgl-distilbert" {
		t.Fatalf("Predict() = %+v", result)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestResultFromClassifier(t *testing.T) {
	alert := resultFromClassifier(ClassifierResult{Label: "ALERT", AlertProbability: 0.45, Threshold: 0.25})
	if alert.EdgePred != PredAbnormal || alert.Confidence != 0.45 || alert.Severity < 70 {
		t.Fatalf("alert result = %+v", alert)
	}
	normal := resultFromClassifier(ClassifierResult{Label: "NON_ALERT", AlertProbability: 0.1, Threshold: 0.25})
	if normal.EdgePred != PredNormal || normal.Confidence != 0.9 || normal.Severity > 20 {
		t.Fatalf("normal result = %+v", normal)
	}
}
