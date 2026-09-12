package provider

import "testing"

func TestParseDiagnosisRepairsWrappedJSON(t *testing.T) {
	d, ok := ParseDiagnosis("result:\n{\"pred\":\"net_error\",\"root_cause\":\"port closed\",\"recommendation\":\"check service\",\"explanation\":\"refused\",\"evidence\":[\"connection refused\"]}\nthanks")
	if !ok || d.Pred != "NET_ERROR" || d.Confidence <= 0 {
		t.Fatalf("unexpected parse: ok=%v diagnosis=%+v", ok, d)
	}
}

func TestParseDiagnosisFallback(t *testing.T) {
	d, ok := ParseDiagnosis("not json")
	if ok || d.Pred != "UNKNOWN" || d.Confidence != .3 {
		t.Fatalf("unexpected fallback: ok=%v diagnosis=%+v", ok, d)
	}
}
