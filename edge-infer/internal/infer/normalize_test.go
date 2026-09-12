package infer

import "testing"

func TestNormalize(t *testing.T) {
	input := "2026-06-22T03:00:00Z [pid=123] connect to 10.0.0.2:3306 at /var/lib/mysql/data"
	got := Normalize(input, 8192)
	want := "connect to <IP>:<NUM> at <PATH>"
	if got != want {
		t.Fatalf("Normalize() = %q, want %q", got, want)
	}
}

func TestTemplateIDIsStable(t *testing.T) {
	first := TemplateID("connection refused <IP>:<NUM>")
	second := TemplateID("connection refused <IP>:<NUM>")
	if first != second {
		t.Fatalf("TemplateID() is not stable: %q != %q", first, second)
	}
	if len(first) != 16 {
		t.Fatalf("TemplateID() length = %d, want 16", len(first))
	}
}
