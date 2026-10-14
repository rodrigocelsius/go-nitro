package queue

import (
	"container/heap"
	"sync"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

type queueItem struct {
	job    domain.Job
	config domain.JobConfig
	id     string
	index  int // index no heap
}

// priorityQueue implements heap.Interface for priority.
type priorityQueue []*queueItem

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].config.Priority > pq[j].config.Priority // higher priority first
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*queueItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[:n-1]
	return item
}

type memoryQueue struct {
	mu    sync.Mutex
	pq    priorityQueue
	items map[string]*queueItem
}

// NewMemoryQueue creates an in-memory queue with priority support.
func NewMemoryQueue() *memoryQueue {
	q := &memoryQueue{
		items: make(map[string]*queueItem),
	}
	heap.Init(&q.pq)
	return q
}

func (q *memoryQueue) Enqueue(job domain.Job) (string, error) {
	return q.EnqueueWithConfig(job, domain.DefaultJobConfig())
}

func (q *memoryQueue) EnqueueWithConfig(job domain.Job, config domain.JobConfig) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	id := uuid.NewString()
	item := &queueItem{
		job:    job,
		config: config,
		id:     id,
	}
	q.items[id] = item
	heap.Push(&q.pq, item)
	return id, nil
}

func (q *memoryQueue) Dequeue() (domain.Job, domain.JobConfig, string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.pq.Len() == 0 {
		return nil, domain.JobConfig{}, "", domain.ErrEmptyQueue{}
	}

	item := heap.Pop(&q.pq).(*queueItem)
	delete(q.items, item.id)
	return item.job, item.config, item.id, nil
}

func (q *memoryQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pq.Len()
}
