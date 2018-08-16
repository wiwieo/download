package pool_new

import (
	"fmt"
	"time"
)

const (
	FACTORY_SCALE = 1000
)

var (
	workerCount             = 0
	addToPool   chan worker = make(chan worker, FACTORY_SCALE)
)

// 任务
type job struct {
	desc   string                           // 工作描述
	do     func(...interface{}) interface{} // 工作
	params []interface{}                    // 材料
	result chan interface{}
}

//
type worker struct {
	id   int
	name string
	jobs chan job
	busy chan bool
}

func newWork() *worker {
	return &worker{
		id:   workerCount,
		name: fmt.Sprintf("%d-%d", time.Now().UnixNano(), workerCount),
		jobs: make(chan job, 1),
		busy: make(chan bool, 1),
	}
}

func (w *worker) work() {
	go func() {
		select {
		case j := <-w.jobs:
			rst := j.do(j.params...)
			if rst != nil {
				j.result <- rst
			}
			addToPool <- *w
		case <-time.After(10 * time.Second):
			fmt.Println("超时了")
		}
	}()
}

func (w *worker) recent(j job) {
	go func() { w.jobs <- j }()
	w.work()
}

//
type Factory struct {
	worker chan worker //
	jobs   chan job
}

func NewFactoryAndRun() *Factory {
	f := &Factory{
		worker: make(chan worker, FACTORY_SCALE),
		jobs:   make(chan job, FACTORY_SCALE),
	}
	f.Run()

	go func() {
		for {
			w := <-addToPool
			f.worker <- w
		}
	}()
	return f
}

func (f *Factory) Recent(name string, do func(...interface{}) interface{}, result chan interface{}, params ...interface{}) {
	go func() {
		j := job{
			do:     do,
			params: params,
			result: result,
		}
		f.jobs <- j
	}()
}

func (f *Factory) Run() {
	go func() {
		for {
			select {
			case j := <-f.jobs:
				select {
				case w := <-f.worker:
					w.recent(j)
				default:
					w := newWork()
					w.recent(j)
				}
			}
		}
	}()
}
