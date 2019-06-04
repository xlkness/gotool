package time_heap

import (
	"container/heap"
	"golang.org/x/net/context"
	"sync"
	"time"
)

type Heap struct {
	timers          *timersHeap
	mu              *sync.Mutex
	chWakeUp        chan struct{}
	workerPoolNum   int
	taskDeliverChan chan *Task
	// 停止
	cancelFun context.CancelFunc
}

func NewAndRun(workerPoolNum int) *Heap {
	h := &Heap{}
	h.timers = newTimersHeap()
	h.chWakeUp = make(chan struct{}, 1)
	h.workerPoolNum = workerPoolNum
	h.taskDeliverChan = make(chan *Task, workerPoolNum*2)
	ctx, cancelFun := context.WithCancel(context.Background())
	h.cancelFun = cancelFun
	startWorkerPool(workerPoolNum, h.taskDeliverChan, ctx)
	return h
}

func (h *Heap) Add(et ExpireType, timeout time.Duration, t *Task) int {
	idx := 0
	h.mu.Lock()
	heap.Push(h.timers, newTimer(et, timeout, t))
	idx = h.timers.timers[len(h.timers.timers)].id
	h.mu.Unlock()
	h.wakeup()
	return idx
}

func (h *Heap) Remove(idx int) {
	h.mu.Lock()
	if idx != -1 {
		heap.Remove(h.timers, idx)
	}
	h.mu.Unlock()
}

func (h *Heap) Run() {
	boomTimerFun := func() (time.Duration, int) {
		h.mu.Lock()
		hlen := h.timers.Len()
		now := time.Now()
		for i := 0; i < hlen; i++ {
			entry := &h.timers.timers[0]
			if now.After(entry.ts) {
				// 执行update
				h.taskDeliverChan <- entry.t
				// 设置堆元素下次到时时间
				if entry.exType == ExpireEvery {
					entry.ts = now.Add(entry.dura)
				} else {
					heap.Remove(h.timers, entry.id)
				}
				// 调整小根堆
				heap.Fix(h.timers, 0)
			} else {
				// 小根堆堆顶没有能超时的定时器，停止检索
				break
			}
		}
		var nextBoomTime time.Duration

		// 小根堆调整完毕，等待下次最近超时，
		// 下次最近超时即为堆顶
		hlen = h.timers.Len()
		if hlen > 0 {
			nextBoomTime = h.timers.timers[0].ts.Sub(now)
		}
		h.mu.Unlock()
		return nextBoomTime, hlen
	}

	// 触发第一个定时事件
	var nextBoomTime time.Duration
	var hlen int
	for {
		select {
		case <-h.chWakeUp:
		}

		// 寻找有无能超时的
		nextBoomTime, hlen = boomTimerFun()
		if hlen <= 0 {
			// 时间堆没有定时器了，继续等待下次有定时器加入唤醒
			continue
		} else {
			// 时间堆有定时器，启动定时器超时
			break
		}

	}

	ticker := time.NewTimer(nextBoomTime)
	defer ticker.Stop()

	scr := false
	for {
		ret := ticker.Stop()
		if !ret && !scr {
			<-ticker.C
		}
		if hlen > 0 {
			ticker.Reset(nextBoomTime)
		}
		select {
		case <-ticker.C:
			scr = true
		case <-h.chWakeUp:
		}

		// 检查时间堆并执行超时动作
		nextBoomTime, hlen = boomTimerFun()
	}
}

func (h *Heap) wakeup() {
	select {
	case h.chWakeUp <- struct{}{}:
	default:
	}
}

type timersHeap struct {
	timers []timer
}

func newTimersHeap() *timersHeap {
	return &timersHeap{
		timers: make([]timer, 0),
	}
}

func (h *timersHeap) Len() int           { return len(h.timers) }
func (h *timersHeap) Less(i, j int) bool { return h.timers[i].ts.Before(h.timers[j].ts) }
func (h *timersHeap) Swap(i, j int) {
	h.timers[i], h.timers[j] = h.timers[j], h.timers[i]
	h.timers[i].id = i
	h.timers[j].id = j
}

func (h *timersHeap) Push(x interface{}) {
	h.timers = append(h.timers, x.(timer))
	n := len(h.timers)
	h.timers[n-1].id = n - 1
}

func (h *timersHeap) Pop() interface{} {
	n := len(h.timers)
	x := h.timers[n-1]
	h.timers[n-1].id = -1
	h.timers[n-1] = timer{}
	h.timers = h.timers[0 : n-1]
	return x
}
