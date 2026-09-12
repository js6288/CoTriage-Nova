package runtime

import (
	"context"
	"net/http"
	"time"
)

const (
	CloudHealthUp   = "UP"
	CloudHealthDown = "DOWN"
)

type HealthSample struct {
	OK    bool          `json:"ok"`
	RTT   time.Duration `json:"rtt"`
	Error string        `json:"error,omitempty"`
}

type HealthProbe struct {
	URL     string
	Window  int
	Client  *http.Client
	samples []HealthSample
}

func NewHealthProbe(url string, timeout time.Duration, window int) *HealthProbe {
	if window <= 0 {
		window = 20
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &HealthProbe{
		URL: url, Window: window,
		Client: &http.Client{Timeout: timeout},
	}
}

func (p *HealthProbe) Probe(ctx context.Context) HealthSample {
	if ctx == nil {
		ctx = context.Background()
	}
	if p.URL == "" {
		sample := HealthSample{OK: true, RTT: 0}
		p.Add(sample)
		return sample
	}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		sample := HealthSample{OK: false, Error: err.Error()}
		p.Add(sample)
		return sample
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		sample := HealthSample{OK: false, RTT: time.Since(start), Error: err.Error()}
		p.Add(sample)
		return sample
	}
	defer resp.Body.Close()
	ok := resp.StatusCode >= 200 && resp.StatusCode < 500
	sample := HealthSample{OK: ok, RTT: time.Since(start)}
	p.Add(sample)
	return sample
}

func (p *HealthProbe) Add(sample HealthSample) {
	p.samples = append(p.samples, sample)
	if len(p.samples) > p.Window {
		p.samples = p.samples[len(p.samples)-p.Window:]
	}
}

func (p *HealthProbe) Snapshot() (health string, rttMS float64, failRate float64) {
	if len(p.samples) == 0 {
		return CloudHealthUp, 0, 0
	}
	failures := 0
	totalRTT := time.Duration(0)
	for _, sample := range p.samples {
		if !sample.OK {
			failures++
		}
		totalRTT += sample.RTT
	}
	failRate = float64(failures) / float64(len(p.samples))
	health = CloudHealthUp
	if failRate >= 0.5 {
		health = CloudHealthDown
	}
	rttMS = float64(totalRTT.Milliseconds()) / float64(len(p.samples))
	return health, rttMS, failRate
}
