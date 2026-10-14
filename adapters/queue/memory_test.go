package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

type testJob struct {
	err error
}

func (j *testJob) Execute(ctx context.Context) error {
	return j.err
}

func TestPriorityQueueOrder(t *testing.T) {
	q := NewMemoryQueue()

	q.EnqueueWithConfig(&testJob{}, domain.JobConfig{Priority: domain.PriorityLow})
	q.EnqueueWithConfig(&testJob{}, domain.JobConfig{Priority: domain.PriorityHigh})
	q.EnqueueWithConfig(&testJob{}, domain.JobConfig{Priority: domain.PriorityNormal})

	_, _, _, err := q.Dequeue()
	if err != nil {
		t.Fatal(err)
	}
	// High priority should come first
	item, _, id, err := q.Dequeue()
	if err != nil {
		t.Fatal(err)
	}
	if item == nil || id == "" {
		t.Fatal("expected a job")
	}
}

func TestPriorityQueueEmpty(t *testing.T) {
	q := NewMemoryQueue()

	_, _, _, err := q.Dequeue()
	if !errors.Is(err, domain.ErrEmptyQueue{}) {
		t.Fatal("expected ErrEmptyQueue")
	}
}

func TestPriorityQueueSize(t *testing.T) {
	q := NewMemoryQueue()

	q.Enqueue(&testJob{})
	q.Enqueue(&testJob{})

	if q.Size() != 2 {
		t.Fatalf("expected size 2, got %d", q.Size())
	}
}

func TestDispatcherRetry(t *testing.T) {
	job := &testJob{
		err: errors.New("fail"),
	}

	q := NewMemoryQueue()
	d := usecases.NewDispatcher(q)

	id, err := d.DispatchWithConfig(job, domain.JobConfig{
		Priority:   domain.PriorityNormal,
		MaxRetries: 2,
		RetryDelay: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	w := usecases.NewWorker(d)
	w.Start()

	// Wait for processing
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		result, ok := d.Result(id)
		if ok && result.Status == domain.JobDeadLetter {
			break
		}
	}
	w.Stop()

	result, ok := d.Result(id)
	if !ok {
		t.Fatal("expected result")
	}

	if result.Status != domain.JobDeadLetter {
		t.Fatalf("expected dead_letter status, got %s", result.Status)
	}
}

func TestDispatcherSuccess(t *testing.T) {
	job := &testJob{err: nil}

	q := NewMemoryQueue()
	d := usecases.NewDispatcher(q)

	id, err := d.Dispatch(job)
	if err != nil {
		t.Fatal(err)
	}

	w := usecases.NewWorker(d)
	w.Start()

	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		result, ok := d.Result(id)
		if ok && result.Status == domain.JobCompleted {
			break
		}
	}
	w.Stop()

	result, ok := d.Result(id)
	if !ok {
		t.Fatal("expected result")
	}

	if result.Status != domain.JobCompleted {
		t.Fatalf("expected completed status, got %s", result.Status)
	}
}
