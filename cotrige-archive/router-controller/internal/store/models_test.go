package store

import "testing"

func TestTableNames(t *testing.T) {
	cases := map[string]string{
		"decision": RouterDecisionModel{}.TableName(),
		"snapshot": RouterStateSnapshotModel{}.TableName(),
		"outbox":   CloudOutboxModel{}.TableName(),
		"feedback": CloudFeedbackModel{}.TableName(),
	}
	want := map[string]string{
		"decision": "router_decisions",
		"snapshot": "router_state_snapshots",
		"outbox":   "cloud_outbox",
		"feedback": "cloud_feedback",
	}
	for name, got := range cases {
		if got != want[name] {
			t.Fatalf("%s table name = %q, want %q", name, got, want[name])
		}
	}
}

func TestOutboxStatusConstants(t *testing.T) {
	if OutboxStatusPending != "PENDING" || OutboxStatusDLQ != "DLQ" {
		t.Fatalf("unexpected outbox status constants")
	}
}
