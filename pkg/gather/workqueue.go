// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import "sync"

type WorkFunc func() error

type Queuer interface {
	Queue(WorkFunc)
}

type WorkQueue struct {
	queue   chan WorkFunc
	workers int
	wg      sync.WaitGroup
	mutex   sync.Mutex
	err     error
	closed  bool
}

func NewWorkQueue(workers int) *WorkQueue {
	return &WorkQueue{
		queue:   make(chan WorkFunc),
		workers: workers,
	}
}

func (q *WorkQueue) Queue(work WorkFunc) {
	q.queue <- work
}

func (q *WorkQueue) Start() {
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go func() {
			defer q.wg.Done()
			for work := range q.queue {
				err := work()
				if err != nil {
					q.setFirstError(err)
				}
			}
		}()
	}
}

func (q *WorkQueue) Wait() error {
	q.wg.Wait()
	return q.firstError()
}

func (q *WorkQueue) Close() {
	// Closing closed channel panics, so we must call it exactly once.
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if !q.closed {
		close(q.queue)
		q.closed = true
	}
}

func (q *WorkQueue) firstError() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.err
}

func (q *WorkQueue) setFirstError(err error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if q.err == nil {
		q.err = err
	}
}
