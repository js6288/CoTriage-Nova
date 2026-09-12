package runtime

import "time"

type TokenBucket struct {
	Capacity float64   `json:"capacity"`
	Tokens   float64   `json:"tokens"`
	Refill   float64   `json:"refill"`
	Last     time.Time `json:"last"`
}

func NewTokenBucket(capacity, refill float64, now time.Time) TokenBucket {
	if capacity < 0 {
		capacity = 0
	}
	if refill < 0 {
		refill = 0
	}
	return TokenBucket{Capacity: capacity, Tokens: capacity, Refill: refill, Last: now}
}

func (b *TokenBucket) RefillTo(now time.Time) {
	if b.Last.IsZero() {
		b.Last = now
	}
	if now.Before(b.Last) {
		return
	}
	elapsed := now.Sub(b.Last).Seconds()
	b.Tokens += elapsed * b.Refill
	if b.Tokens > b.Capacity {
		b.Tokens = b.Capacity
	}
	b.Last = now
}

func (b *TokenBucket) Allow(cost float64, now time.Time) bool {
	if cost <= 0 {
		cost = 1
	}
	b.RefillTo(now)
	if b.Tokens < cost {
		return false
	}
	b.Tokens -= cost
	return true
}

type TokenBuckets struct {
	Minute TokenBucket `json:"minute"`
	Hour   TokenBucket `json:"hour"`
	Day    TokenBucket `json:"day"`
	Rate   TokenBucket `json:"rate"`
}

func NewTokenBuckets(minute, hour, day float64, rate int, now time.Time) TokenBuckets {
	return TokenBuckets{
		Minute: NewTokenBucket(minute, minute/60, now),
		Hour:   NewTokenBucket(hour, hour/3600, now),
		Day:    NewTokenBucket(day, day/86400, now),
		Rate:   NewTokenBucket(float64(rate), float64(rate)/60, now),
	}
}

func (b *TokenBuckets) RefillTo(now time.Time) {
	b.Minute.RefillTo(now)
	b.Hour.RefillTo(now)
	b.Day.RefillTo(now)
	b.Rate.RefillTo(now)
}

func (b *TokenBuckets) Allow(cost float64, now time.Time) bool {
	b.RefillTo(now)
	if b.Minute.Tokens < cost || b.Hour.Tokens < cost || b.Day.Tokens < cost || b.Rate.Tokens < cost {
		return false
	}
	b.Minute.Tokens -= cost
	b.Hour.Tokens -= cost
	b.Day.Tokens -= cost
	b.Rate.Tokens -= cost
	return true
}
