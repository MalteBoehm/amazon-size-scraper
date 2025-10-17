package queue

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrQueueEmpty  = errors.New("queue is empty")
	ErrQueueClosed = errors.New("queue is closed")
)

type Task struct {
	ID        string
	URL       string
	ASIN      string
	Priority  int
	Retries   int
	CreatedAt time.Time
}

type Queue interface {
	Push(task *Task) error
	Pop(ctx context.Context) (*Task, error)
	Size() int
	Close() error
}

type InMemoryQueue struct {
	tasks  []*Task
	mu     sync.Mutex
	cond   *sync.Cond
	closed bool
}

func NewInMemoryQueue() *InMemoryQueue {
	q := &InMemoryQueue{
		tasks: make([]*Task, 0),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *InMemoryQueue) Push(task *Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if q.closed {
		return ErrQueueClosed
	}
	
	q.tasks = append(q.tasks, task)
	q.sortByPriority()
	q.cond.Signal()
	
	return nil
}

func (q *InMemoryQueue) Pop(ctx context.Context) (*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	for len(q.tasks) == 0 && !q.closed {
		done := make(chan struct{})
		go func() {
			q.cond.Wait()
			close(done)
		}()
		
		select {
		case <-ctx.Done():
			q.cond.Signal()
			return nil, ctx.Err()
		case <-done:
		}
	}
	
	if q.closed && len(q.tasks) == 0 {
		return nil, ErrQueueClosed
	}
	
	if len(q.tasks) == 0 {
		return nil, ErrQueueEmpty
	}
	
	task := q.tasks[0]
	q.tasks = q.tasks[1:]
	
	return task, nil
}

func (q *InMemoryQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tasks)
}

func (q *InMemoryQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	q.closed = true
	q.cond.Broadcast()
	
	return nil
}

func (q *InMemoryQueue) sortByPriority() {
	for i := 0; i < len(q.tasks)-1; i++ {
		for j := 0; j < len(q.tasks)-i-1; j++ {
			if q.tasks[j].Priority < q.tasks[j+1].Priority {
				q.tasks[j], q.tasks[j+1] = q.tasks[j+1], q.tasks[j]
			}
		}
	}
}

type BatchQueue struct {
	queue     Queue
	batchSize int
}

func NewBatchQueue(q Queue, batchSize int) *BatchQueue {
	return &BatchQueue{
		queue:     q,
		batchSize: batchSize,
	}
}

func (b *BatchQueue) PushBatch(tasks []*Task) error {
	for _, task := range tasks {
		if err := b.queue.Push(task); err != nil {
			return err
		}
	}
	return nil
}

func (b *BatchQueue) PopBatch(ctx context.Context) ([]*Task, error) {
	var tasks []*Task
	
	for i := 0; i < b.batchSize; i++ {
		task, err := b.queue.Pop(ctx)
		if err != nil {
			if err == ErrQueueEmpty || err == ErrQueueClosed {
				break
			}
			return tasks, err
		}
		tasks = append(tasks, task)
	}
	
	if len(tasks) == 0 {
		return nil, ErrQueueEmpty
	}
	
	return tasks, nil
}