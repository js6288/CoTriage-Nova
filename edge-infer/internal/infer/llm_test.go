package infer

import "testing"

func TestParseLLMJSON(t *testing.T) {
	result, err := parseLLMJSON("```json\n{\"edge_pred\":\"NET_ERROR\",\"severity\":78,\"confidence\":0.72,\"evidence\":[\"connection refused\"],\"advice_code\":\"CHECK_NETWORK\",\"advice\":\"检查服务端口\"}\n```")
	if err != nil {
		t.Fatalf("parseLLMJSON() error = %v", err)
	}
	if result.EdgePred != PredNetError || result.Severity != 78 || result.Confidence != 0.72 || result.AdviceCode != "CHECK_NETWORK" {
		t.Fatalf("parseLLMJSON() = %+v", result)
	}
}

func TestParseLLMJSONRejectsInvalidPrediction(t *testing.T) {
	_, err := parseLLMJSON(`{"edge_pred":"ABNORMAL","severity":75,"confidence":0.7}`)
	if err == nil {
		t.Fatal("parseLLMJSON() error = nil, want invalid prediction error")
	}
}

func TestParseLLMJSONRepairsPartialOutput(t *testing.T) {
	result, err := parseLLMJSON(`{"edge_pred":"NET_ERROR","severity":78,"confidence":0.72,"evidence":["connection refused"],"advice_code":"CHECK_NETWORK","advice":"检查`)
	if err != nil {
		t.Fatalf("parseLLMJSON() error = %v", err)
	}
	if result.EdgePred != PredNetError || result.Severity != 78 || result.Confidence != 0.72 {
		t.Fatalf("parseLLMJSON() = %+v", result)
	}
	if len(result.Evidence) == 0 || result.Evidence[len(result.Evidence)-1] != "llm_json_repaired" {
		t.Fatalf("Evidence = %+v, want repair marker", result.Evidence)
	}
}

func TestResultFromLLM(t *testing.T) {
	got := resultFromLLM(LLMResult{
		EdgePred: PredDiskError, Severity: 91, Confidence: 0.8,
		Evidence: []string{"no space"}, AdviceCode: "CHECK_DISK", Advice: "检查磁盘",
	})
	if got.EdgePred != PredDiskError || got.Severity != 91 || got.Confidence != 0.8 || got.AdviceCode != "CHECK_DISK" {
		t.Fatalf("resultFromLLM() = %+v", got)
	}
}
