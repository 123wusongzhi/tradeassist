package taskretry

import (
	"testing"
	"time"
)

func TestPolicy_NextRunAtBackoff(t *testing.T) {
	t.Parallel()
	p := DefaultPolicy()
	t1 := p.NextRunAt(2, 0)
	t2 := p.NextRunAt(3, 0)
	if t2.Before(t1) {
		t.Fatal("attempt 3 should be later than attempt 2")
	}
}

func TestPolicy_RetryAfter(t *testing.T) {
	t.Parallel()
	p := DefaultPolicy()
	at := p.NextRunAt(2, 45*time.Second)
	if at.Sub(time.Now().UTC()) < 40*time.Second {
		t.Fatal("expected retry-after delay")
	}
}

func TestPolicy_DeadLetter(t *testing.T) {
	t.Parallel()
	p := DefaultPolicy()
	if !p.IsDeadLetter(5) {
		t.Fatal("attempt 5 should be dead letter")
	}
	if p.ShouldRetry(5, true) {
		t.Fatal("should not retry at max attempts")
	}
}

func TestClassify_retryable(t *testing.T) {
	t.Parallel()
	cls := Classify(errTimeout{}, 0)
	if !cls.Retryable || cls.Code != CodeTimeout {
		t.Fatalf("expected timeout retryable, got %+v", cls)
	}
	cls2 := Classify(errValidation{}, 400)
	if cls2.Retryable {
		t.Fatalf("validation should be permanent, got %+v", cls2)
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()
	sec, ok := ParseRetryAfter("120")
	if !ok || sec != 120 {
		t.Fatalf("expected 120, got %d ok=%v", sec, ok)
	}
}

type errTimeout struct{}

func (errTimeout) Error() string { return "request timeout" }

type errValidation struct{}

func (errValidation) Error() string { return "validation failed: bad input" }
