package auth

import (
	"strings"
	"sync"
	"time"
)

// Throttle slows down password guessing: 5 failures per account and IP, 20 per IP, within 15 minutes.
// In memory on purpose – one instance, and a restart only resets the counters.
type Throttle struct {
	mu         sync.Mutex
	window     time.Duration
	perAccount int
	perIP      int
	now        func() time.Time
	hits       map[string]*bucket
}

type bucket struct {
	count int
	start time.Time
}

func NewThrottle() *Throttle {
	return &Throttle{window: 15 * time.Minute, perAccount: 5, perIP: 20, now: time.Now, hits: map[string]*bucket{}}
}

func keys(ip, email string) (string, string) {
	return "ip:" + ip, "acct:" + ip + "|" + strings.ToLower(strings.TrimSpace(email))
}

func (t *Throttle) Allow(ip, email string) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ipKey, acctKey := keys(ip, email)
	for key, limit := range map[string]int{ipKey: t.perIP, acctKey: t.perAccount} {
		if b := t.live(key); b != nil && b.count >= limit {
			return false, b.start.Add(t.window).Sub(t.now())
		}
	}
	return true, 0
}

// Reserve checks the limits and counts the attempt before the password is verified, so parallel requests
// cannot all slip past the limit. A failed login keeps the reservation; Success and Release give it back.
func (t *Throttle) Reserve(ip, email string) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ipKey, acctKey := keys(ip, email)
	for key, limit := range map[string]int{ipKey: t.perIP, acctKey: t.perAccount} {
		if b := t.live(key); b != nil && b.count >= limit {
			return false, b.start.Add(t.window).Sub(t.now())
		}
	}
	if len(t.hits) > 10000 {
		t.prune()
	}
	t.add(ipKey)
	t.add(acctKey)
	return true, 0
}

// Release gives back a reservation without counting a failure (e.g. when the database was unavailable).
func (t *Throttle) Release(ip, email string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ipKey, acctKey := keys(ip, email)
	t.sub(ipKey)
	t.sub(acctKey)
}

func (t *Throttle) add(key string) {
	if b := t.live(key); b != nil {
		b.count++
	} else {
		t.hits[key] = &bucket{count: 1, start: t.now()}
	}
}

func (t *Throttle) sub(key string) {
	if b := t.live(key); b != nil {
		if b.count--; b.count <= 0 {
			delete(t.hits, key)
		}
	}
}

func (t *Throttle) Fail(ip, email string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.hits) > 10000 {
		t.prune()
	}
	ipKey, acctKey := keys(ip, email)
	for _, key := range []string{ipKey, acctKey} {
		if b := t.live(key); b != nil {
			b.count++
		} else {
			t.hits[key] = &bucket{count: 1, start: t.now()}
		}
	}
}

func (t *Throttle) Success(ip, email string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ipKey, acctKey := keys(ip, email)
	delete(t.hits, acctKey)
	t.sub(ipKey) // the IP counter only keeps failures, so the reservation of this attempt goes back
}

func (t *Throttle) live(key string) *bucket {
	b := t.hits[key]
	if b == nil || t.now().Sub(b.start) >= t.window {
		delete(t.hits, key)
		return nil
	}
	return b
}

func (t *Throttle) prune() {
	for key := range t.hits {
		t.live(key)
	}
}
