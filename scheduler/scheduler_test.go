package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseSchedule_Interval(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"5s", 5 * time.Second},
		{"5m", 5 * time.Minute},
		{"1h", 1 * time.Hour},
		{"24h", 24 * time.Hour},
		{"1d", 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			schedule, err := ParseSchedule(tt.input)
			if err != nil {
				t.Fatalf("ParseSchedule(%q) failed: %v", tt.input, err)
			}

			interval, ok := schedule.(*IntervalSchedule)
			if !ok {
				t.Fatalf("expected *IntervalSchedule, got %T", schedule)
			}

			if interval.Interval != tt.expected {
				t.Errorf("expected interval %v, got %v", tt.expected, interval.Interval)
			}
		})
	}
}

func TestParseSchedule_Cron(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"*/5 * * * *", true},
		{"0 * * * *", true},
		{"0 0 * * *", true},
		{"0 0 1 * *", true},
		{"0 0 * * 0", true},
		{"*", false},
		{"* *", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := ParseSchedule(tt.input)
			if tt.valid && err != nil {
				t.Errorf("ParseSchedule(%q) should succeed: %v", tt.input, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("ParseSchedule(%q) should fail", tt.input)
			}
		})
	}
}

func TestCronSchedule_Next(t *testing.T) {
	schedule, err := ParseSchedule("*/5 * * * *")
	if err != nil {
		t.Fatalf("ParseSchedule failed: %v", err)
	}

	// Test from a specific hour
	base := time.Date(2024, 1, 1, 10, 3, 0, 0, time.UTC)
	next := schedule.Next(base)

	// Should be 10:05
	expected := time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected next %v, got %v", expected, next)
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s := New()

	var count int32
	if _, err := s.Every("1s", func(ctx context.Context) error {
		atomic.AddInt32(&count, 1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for a few runs
	time.Sleep(2500 * time.Millisecond)

	if err := s.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if atomic.LoadInt32(&count) < 2 {
		t.Errorf("expected at least 2 executions, got %d", atomic.LoadInt32(&count))
	}
}

func TestScheduler_MultipleJobs(t *testing.T) {
	s := New()

	var count1, count2 int32
	if _, err := s.Every("1s", func(ctx context.Context) error {
		atomic.AddInt32(&count1, 1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Every("1s", func(ctx context.Context) error {
		atomic.AddInt32(&count2, 1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(2500 * time.Millisecond)

	if err := s.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if atomic.LoadInt32(&count1) < 2 {
		t.Errorf("expected job 1 to run at least 2 times, got %d", atomic.LoadInt32(&count1))
	}

	if atomic.LoadInt32(&count2) < 2 {
		t.Errorf("expected job 2 to run at least 2 times, got %d", atomic.LoadInt32(&count2))
	}
}

func TestScheduler_Jobs(t *testing.T) {
	s := New()

	if _, err := s.Every("1h", func(ctx context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Every("2h", func(ctx context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}

	jobs := s.Jobs()
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestScheduler_InvalidSchedule(t *testing.T) {
	s := New()
	_, err := s.Every("not-a-schedule", func(ctx context.Context) error { return nil })
	if err == nil {
		t.Fatal("expected error for invalid schedule")
	}
}

func TestScheduler_PanicRecovery(t *testing.T) {
	s := New()

	var count int32
	// Job sempre entra em pânico: o scheduler precisa continuar agendando.
	if _, err := s.Every("1s", func(ctx context.Context) error {
		atomic.AddInt32(&count, 1)
		panic("job exploded")
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	time.Sleep(2500 * time.Millisecond)
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if atomic.LoadInt32(&count) < 2 {
		t.Errorf("expected job to keep running after panics, got %d runs", atomic.LoadInt32(&count))
	}
}

func TestScheduler_StopTimeout(t *testing.T) {
	s := New()
	block := make(chan struct{})
	started := make(chan struct{})
	// Job que trava na primeira execução: Stop deve retornar por timeout
	// em vez de pendurar para sempre.
	if _, err := s.Every("1s", func(ctx context.Context) error {
		close(started)
		<-block
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("job did not start")
	}

	start := time.Now()
	err := s.Stop()
	elapsed := time.Since(start)
	close(block)

	if err == nil {
		t.Fatal("expected stop timeout error for stuck job")
	}
	if elapsed > 15*time.Second {
		t.Fatalf("stop took too long: %v", elapsed)
	}
}
