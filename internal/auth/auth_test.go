package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/omarfourati-dev/briefklar/internal/store"
)

func TestTokensRoundTrip(t *testing.T) {
	tok, err := NewTokens([]byte(strings.Repeat("k", 32)), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	s, exp, err := tok.Issue(store.User{ID: "u1", Email: "a@b.de", Name: "Anna", Role: "admin"})
	if err != nil || time.Until(exp) < 59*time.Minute {
		t.Fatalf("issue: %v %v", exp, err)
	}
	c, err := tok.Parse(s)
	if err != nil || c != (Claims{UserID: "u1", Email: "a@b.de", Name: "Anna", Role: "admin"}) {
		t.Fatalf("parse: %+v %v", c, err)
	}
}

func TestTokensRejectTamperingAndExpiry(t *testing.T) {
	tok, _ := NewTokens([]byte(strings.Repeat("k", 32)), time.Hour)
	other, _ := NewTokens([]byte(strings.Repeat("x", 32)), time.Hour)
	s, _, _ := other.Issue(store.User{ID: "u1"})
	if _, err := tok.Parse(s); err == nil {
		t.Fatal("token signed with another key accepted")
	}
	tok.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	old, _, _ := tok.Issue(store.User{ID: "u1"})
	tok.now = time.Now
	if _, err := tok.Parse(old); err == nil {
		t.Fatal("expired token accepted")
	}
	if _, err := NewTokens([]byte("short"), time.Hour); err == nil {
		t.Fatal("short secret accepted")
	}
}

func TestThrottle(t *testing.T) {
	th := NewThrottle()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	th.now = func() time.Time { return now }
	for i := 0; i < 5; i++ {
		th.Fail("1.1.1.1", "a@b.de")
	}
	if ok, retry := th.Allow("1.1.1.1", "A@B.de"); ok || retry <= 0 {
		t.Fatal("6th attempt for the same account must be blocked")
	}
	if ok, _ := th.Allow("1.1.1.1", "other@b.de"); !ok {
		t.Fatal("other account from the same IP is still allowed (5 < 20)")
	}
	for i := 0; i < 15; i++ {
		th.Fail("1.1.1.1", "x"+string(rune('a'+i))+"@b.de")
	}
	if ok, _ := th.Allow("1.1.1.1", "fresh@b.de"); ok {
		t.Fatal("20 failures per IP must block the IP")
	}
	now = now.Add(16 * time.Minute)
	if ok, _ := th.Allow("1.1.1.1", "a@b.de"); !ok {
		t.Fatal("window over, must be allowed again")
	}
}

func TestThrottleSuccessResetsAccount(t *testing.T) {
	th := NewThrottle()
	for i := 0; i < 4; i++ {
		th.Fail("2.2.2.2", "a@b.de")
	}
	th.Success("2.2.2.2", "a@b.de")
	for i := 0; i < 4; i++ {
		th.Fail("2.2.2.2", "a@b.de")
	}
	if ok, _ := th.Allow("2.2.2.2", "a@b.de"); !ok {
		t.Fatal("success must reset the account counter")
	}
}
