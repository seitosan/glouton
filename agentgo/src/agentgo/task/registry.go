package task

import (
	"context"
	"log"
	"sync"
)

// Runner is something that can be Run
type Runner interface {
	Run(context.Context) error
}

// RunCloser is something that can be Run and Close(d)
type RunCloser interface {
	Runner
	Close() error
}

// Registry contains running tasks. It allow to add/remove tasks
type Registry struct {
	ctx             context.Context
	tasks           map[int]Runner
	taskNames       map[int]string
	taskCancelFuncs map[int]func()
	l               sync.Mutex
}

// NewRegistry create a new registry. All task running in this registry will terminate when ctx is cancelled
func NewRegistry(ctx context.Context) *Registry {
	return &Registry{
		ctx:             ctx,
		tasks:           make(map[int]Runner),
		taskNames:       make(map[int]string),
		taskCancelFuncs: make(map[int]func()),
	}
}

// Close stops and wait for all currently running tasks
func (r *Registry) Close() {
	r.l.Lock()
	defer r.l.Unlock()
	for k := range r.taskCancelFuncs {
		r.removeTask(k)
	}
}

// AddTask add and start a new task. It return an taskID that could be used in RemoveTask
func (r *Registry) AddTask(task Runner, shortName string) int {
	r.l.Lock()
	defer r.l.Unlock()

	id := 1
	_, ok := r.taskCancelFuncs[id]
	for ok {
		id++
		if id == 0 {
			panic("too many tasks in the registry. Unable to find new slot")
		}
		_, ok = r.taskCancelFuncs[id]
	}

	ctx, cancel := context.WithCancel(r.ctx)
	waitC := make(chan interface{})
	cancelWait := func() {
		cancel()
		<-waitC
	}
	go func() {
		defer close(waitC)
		err := task.Run(ctx)
		if err != nil {
			log.Printf("Task %#v failed to start: %v", shortName, err)
		}
	}()
	r.taskCancelFuncs[id] = cancelWait
	r.tasks[id] = task
	r.taskNames[id] = shortName

	return id
}

// RemoveTask stop (and potentially close) and remove given task
func (r *Registry) RemoveTask(taskID int) {
	r.l.Lock()
	defer r.l.Unlock()
	r.removeTask(taskID)
}

func (r *Registry) removeTask(taskID int) {
	if cancel, ok := r.taskCancelFuncs[taskID]; ok {
		cancel()
		task := r.tasks[taskID]
		taskName := r.taskNames[taskID]
		if closer, ok := task.(RunCloser); ok {
			if err := closer.Close(); err != nil {
				log.Printf("DBG: Failed to close task %#v: %v", taskName, err)
			}
		}
	} else {
		log.Printf("DBG2: called RemoveTask with unexisting ID %d", taskID)
	}

	delete(r.taskCancelFuncs, taskID)
}