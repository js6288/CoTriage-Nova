package infer

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

type replacement struct {
	pattern *regexp.Regexp
	value   string
}

var normalizers = []replacement{
	{regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`), "<UUID>"},
	{regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`), "<IP>"},
	{regexp.MustCompile(`(?i)\b0x[0-9a-f]+\b`), "<HEX>"},
	{regexp.MustCompile(`(?:/[A-Za-z0-9._-]+){2,}`), "<PATH>"},
	{regexp.MustCompile(`\b\d+\b`), "<NUM>"},
}

var unstablePrefix = regexp.MustCompile(`^\s*(?:\[?\d{4}[-/]\d{2}[-/]\d{2}[T ][^ ]+\]?\s*)?(?:\[?(?:pid|tid|thread)[=: ]?\d+\]?\s*)*`)

func Normalize(raw string, maxBytes int) string {
	raw = strings.TrimSpace(raw)
	if len(raw) > maxBytes {
		raw = raw[:maxBytes]
	}
	normalized := unstablePrefix.ReplaceAllString(raw, "")
	for _, item := range normalizers {
		normalized = item.pattern.ReplaceAllString(normalized, item.value)
	}
	return strings.Join(strings.Fields(normalized), " ")
}

func TemplateID(template string) string {
	sum := sha256.Sum256([]byte(template))
	return hex.EncodeToString(sum[:8])
}
