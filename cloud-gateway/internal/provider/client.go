package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/event"
)

type Config struct {
	ID                                           int64 `json:"id"`
	Provider, Endpoint, Model, APIKeyEnv, APIKey string
	TimeoutMS, MaxTokens                         int
	Temperature, TopP                            float64
	Seed                                         *int
	ResponsePath                                 string
}

func (c *Config) UnmarshalJSON(b []byte) error {
	type raw struct {
		ID           int64   `json:"id"`
		Provider     string  `json:"provider"`
		Endpoint     string  `json:"endpoint"`
		Model        string  `json:"model"`
		APIKeyEnv    string  `json:"api_key_env"`
		APIKey       string  `json:"api_key"`
		TimeoutMS    int     `json:"timeout_ms"`
		MaxTokens    int     `json:"max_tokens"`
		Temperature  float64 `json:"temperature"`
		TopP         float64 `json:"top_p"`
		Seed         *int    `json:"seed"`
		ResponsePath string  `json:"response_path"`
	}
	var r raw
	if e := json.Unmarshal(b, &r); e != nil {
		return e
	}
	*c = Config{r.ID, r.Provider, r.Endpoint, r.Model, r.APIKeyEnv, r.APIKey, r.TimeoutMS, r.MaxTokens, r.Temperature, r.TopP, r.Seed, r.ResponsePath}
	return nil
}

type RemoteConfig struct {
	Enabled       bool    `json:"enabled"`
	Reason        string  `json:"reason"`
	ConfigVersion int64   `json:"config_version"`
	Etag          string  `json:"etag"`
	Provider      *Config `json:"provider"`
	APIKey        string  `json:"api_key"`
}
type Manager struct {
	mu     sync.RWMutex
	cfg    *Config
	etag   string
	url    string
	token  string
	client *http.Client
}

func NewManager(url string, timeout time.Duration, token ...string) *Manager {
	var value string
	if len(token) > 0 {
		value = token[0]
	}
	return &Manager{url: url, token: value, client: &http.Client{Timeout: timeout}}
}
func (m *Manager) Refresh(ctx context.Context) error {
	req, e := http.NewRequestWithContext(ctx, "GET", m.url, nil)
	if e != nil {
		return e
	}
	m.mu.RLock()
	tag := m.etag
	m.mu.RUnlock()
	if tag != "" {
		req.Header.Set("If-None-Match", tag)
	}
	if m.token != "" {
		req.Header.Set("Authorization", "Bearer "+m.token)
	}
	resp, e := m.client.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode == 304 {
		return nil
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("cloud config status %d", resp.StatusCode)
	}
	var c RemoteConfig
	if e = json.NewDecoder(resp.Body).Decode(&c); e != nil {
		return e
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.etag = c.Etag
	if c.Enabled {
		if c.Provider != nil && c.APIKey != "" {
			c.Provider.APIKey = c.APIKey
		}
		m.cfg = c.Provider
	} else {
		m.cfg = nil
	}
	return nil
}
func (m *Manager) Get() (*Config, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cfg == nil {
		return nil, errors.New("PROVIDER_CONFIG_MISSING")
	}
	v := *m.cfg
	return &v, nil
}

type Client struct{}
type providerResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		Prompt     int `json:"prompt_tokens"`
		Completion int `json:"completion_tokens"`
		Total      int `json:"total_tokens"`
	} `json:"usage"`
}

func (Client) Diagnose(ctx context.Context, c Config, system, user string) (event.Diagnosis, event.Usage, error) {
	key := strings.TrimSpace(c.APIKey)
	if key == "" {
		key = os.Getenv(c.APIKeyEnv)
	}
	if key == "" {
		return event.Diagnosis{}, event.Usage{}, errors.New("PROVIDER_CONFIG_MISSING: api key env is empty")
	}
	body := map[string]any{"model": c.Model, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}}, "max_tokens": c.MaxTokens, "temperature": c.Temperature, "top_p": c.TopP, "response_format": map[string]string{"type": "json_object"}}
	if c.Seed != nil {
		body["seed"] = *c.Seed
	}
	b, _ := json.Marshal(body)
	timeout := time.Duration(c.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(callCtx, "POST", c.Endpoint, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	start := time.Now()
	resp, e := http.DefaultClient.Do(req)
	lat := time.Since(start).Milliseconds()
	if e != nil {
		return event.Diagnosis{}, event.Usage{LatencyMS: lat}, fmt.Errorf("LLM_TIMEOUT: %w", e)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode == 429 {
		return event.Diagnosis{}, event.Usage{LatencyMS: lat}, errors.New("RATE_LIMIT")
	}
	if resp.StatusCode >= 500 {
		return event.Diagnosis{}, event.Usage{LatencyMS: lat}, fmt.Errorf("PROVIDER_5XX: status %d", resp.StatusCode)
	}
	if resp.StatusCode >= 300 {
		return event.Diagnosis{}, event.Usage{LatencyMS: lat}, fmt.Errorf("PROVIDER_ERROR: status %d: %s", resp.StatusCode, string(raw))
	}
	var pr providerResponse
	if e = json.Unmarshal(raw, &pr); e != nil || len(pr.Choices) == 0 {
		return event.Diagnosis{}, event.Usage{LatencyMS: lat}, errors.New("JSON_SCHEMA_INVALID: provider response")
	}
	d, structured := ParseDiagnosis(pr.Choices[0].Message.Content)
	u := event.Usage{PromptTokens: pr.Usage.Prompt, CompletionTokens: pr.Usage.Completion, TotalTokens: pr.Usage.Total, LatencyMS: lat}
	if !structured {
		d.Confidence = .3
	}
	return d, u, nil
}

var allowed = map[string]bool{"NORMAL": true, "NET_ERROR": true, "DISK_ERROR": true, "CPU_HIGH": true, "MEMORY_HIGH": true, "SERVICE_ERROR": true, "CONFIG_ERROR": true, "SECURITY_RISK": true, "UNKNOWN": true}

func ParseDiagnosis(s string) (event.Diagnosis, bool) {
	var d event.Diagnosis
	clean := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(s), "```json"), "```"))
	ok := json.Unmarshal([]byte(clean), &d) == nil
	if !ok {
		a := strings.Index(s, "{")
		b := strings.LastIndex(s, "}")
		if a >= 0 && b > a {
			ok = json.Unmarshal([]byte(s[a:b+1]), &d) == nil
		}
	}
	if !ok {
		return event.Diagnosis{Pred: "UNKNOWN", Explanation: truncate(s, 4000), Confidence: .3}, false
	}
	d.Pred = strings.ToUpper(d.Pred)
	if !allowed[d.Pred] {
		d.Pred = "UNKNOWN"
	}
	if d.Confidence <= 0 {
		d.Confidence = .4
		if d.RootCause != "" {
			d.Confidence += .2
		}
		if d.Recommendation != "" {
			d.Confidence += .2
		}
		if len(d.Evidence) > 0 {
			d.Confidence += .1
		}
		if d.Pred != "UNKNOWN" {
			d.Confidence += .1
		}
		if d.Confidence > .95 {
			d.Confidence = .95
		}
	}
	if d.Confidence > 1 {
		d.Confidence = 1
	}
	return d, true
}
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
