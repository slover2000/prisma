package pool

import (
	"fmt"
	"sync"
	"golang.org/x/net/context"
)

// Result is container of result
type Result struct {
	Value interface{}
	Err   error
}

// Task represents callable interface and result channne
type Task struct {
	Callable func() (interface{}, error)
	ResultChan chan *Result
}

// GoPool is a goroutine pool
type GoPool struct {
	concurrency int
	queueSize   int
	tasksChan   chan *Task
	wg          sync.WaitGroup
}

func NewGoPool(concurrency, queueSize int) *GoPool {
	return &GoPool{
		concurrency: concurrency,
		queueSize: queueSize,
		tasksChan: make(chan *Task, queueSize),
	}
}

func (p *GoPool) Start() {
	for i := 0; i < p.concurrency; i++ {		
		go p.work(fmt.Sprintf("worker[%d]", i+1))
	}
}

func (p *GoPool) Stop(ctx context.Context) bool {
    c := make(chan struct{})
    go func() {
        defer close(c)
		p.wg.Wait()
	}()
	
	// close task channel
	close(p.tasksChan)
	p.tasksChan = nil

	select {
	case <-c:
		return true
	case <-ctx.Done():
		return false
	}
}

func (p *GoPool) IsStopped() bool {
	return p.tasksChan == nil
}

// Submit fixed worker pool execute implementation
func (p *GoPool) Submit(task *Task) bool {
	if p.IsStopped() {
		fmt.Println("go pool stopped.\n")
		return false
	}

	select {
	case p.tasksChan <- task:
		return true
	default:
		return false
	}
}

// The work loop for any single goroutine.
func (p *GoPool) work(name string) {
	p.wg.Add(1)
	for task := range p.tasksChan {
		result, err := task.Callable()		
		task.ResultChan <- &Result{Value: result, Err: err}
	}
	p.wg.Done()	
}