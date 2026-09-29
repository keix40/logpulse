package redisx

import "testing"

func TestLogStreamAddArgsApproxMaxLen(t *testing.T) {
	args := LogStreamAddArgs(`{"level":"info"}`, 100)
	if args.Stream != StreamLogs {
		t.Fatalf("stream = %q", args.Stream)
	}
	if args.MaxLen != 100 || !args.Approx {
		t.Fatalf("maxlen/approx = %d/%v", args.MaxLen, args.Approx)
	}
}

func TestLogStreamAddArgsNoTrimWhenZero(t *testing.T) {
	args := LogStreamAddArgs("x", 0)
	if args.MaxLen != 0 || args.Approx {
		t.Fatalf("expected no trimming, got maxlen=%d approx=%v", args.MaxLen, args.Approx)
	}
}
