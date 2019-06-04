package time_wheel

import (
	"container/list"
	"context"
	"sync"
	"time"
)

type customEvent struct {
	op    int32
	key   interface{}
	value interface{}
}

type timersManagerInfo struct {
	slot int32
	elem *list.Element
}

type Wheel struct {
	// 当前槽
	curSlot int32
	// 槽数量
	slotNum int32
	// 滴答间隔
	tickDuration time.Duration
	// 槽
	slots           []*slot
	taskDeliverChan chan *Task
	// 任务工作池数量
	workerPoolNum int32
	// 定时器管理模块
	timersManager *sync.Map
	// 外部事件监听队列
	customEventChan chan *customEvent
	// 停止
	cancelFun context.CancelFunc
}

func NewAndRun(slotNum int32, tick time.Duration, workerPoolNum int32) *Wheel {
	if slotNum <= 0 || tick < time.Second || workerPoolNum <= 0 {
		return nil
	}
	timeWheel := &Wheel{
		slotNum:         slotNum,
		tickDuration:    tick,
		taskDeliverChan: make(chan *Task, workerPoolNum*2),
		slots:           make([]*slot, slotNum),
		workerPoolNum:   workerPoolNum,
		timersManager:   &sync.Map{},
		customEventChan: make(chan *customEvent, 16),
	}
	for i := 0; i < int(slotNum); i++ {
		timeWheel.slots[i] = newSlot()
	}
	timeWheel.run()
	return timeWheel
}

// 停止时间轮
func (tw *Wheel) Stop() {
	tw.cancelFun()
}

func (tw *Wheel) Add(key interface{}, timeout time.Duration, t *Task) {
	addTimer := &timer{timeout: timeout, task: t, timeoutTs: time.Now().Add(timeout).Unix()}
	tw.customEventChan <- &customEvent{
		op:    1,
		key:   key,
		value: addTimer,
	}
}

func (tw *Wheel) Del(key interface{}) {
	tw.customEventChan <- &customEvent{
		op:  2,
		key: key,
	}
}

func (tw *Wheel) run() {
	ctx, cancelFun := context.WithCancel(context.Background())
	tw.cancelFun = cancelFun

	// 启动一定数量的工作池
	startWorkerPool(tw.workerPoolNum, tw.taskDeliverChan, ctx)

	// 启动定时tick协程
	go func() {
		ticker := time.NewTicker(tw.tickDuration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				tw.tick()
			case event := <-tw.customEventChan:
				if event.op == 1 {
					tw.add(event.key, event.value.(*timer))
				} else if event.op == 2 {
					tw.del(event.key)
				}
			case <-ctx.Done():
				//fmt.Printf("time wheel stop..\n")
				return
			}
		}
	}()
}

// 滴答，并超时槽上的定时器
func (tw *Wheel) tick() {
	timeNow := time.Now().Unix()

	// 取当前槽
	curSlot := tw.slots[tw.curSlot]
	tmp := curSlot.front()

	// 遍历前几个能超时的定时器
	for tmp != nil {
		if tmp.timeoutTs <= timeNow {
			tw.deliverTask(tmp.task)
			curSlot.pop()
			tmp = curSlot.front()
			continue
		}
		break
	}

	// 槽索引++
	tw.curSlot++
	tw.curSlot = tw.curSlot % tw.slotNum
}

func (tw *Wheel) add(key interface{}, addTimer *timer) {
	insertSlot := tw.curSlot
	if addTimer.timeout < tw.tickDuration {
		insertSlot = tw.curSlot
	} else {
		// 计算定时器所属槽
		insertSlot = (int32(addTimer.timeout/tw.tickDuration) + tw.curSlot) % tw.slotNum
	}
	//fmt.Printf("cur slot:%v\n", insertSlot)
	// 添加定时器到对应槽
	elem := tw.slots[insertSlot].add(addTimer)

	// 将定时器加入定时器管理模块，作为后续删除的记录
	newTimerInfo := &timersManagerInfo{
		slot: insertSlot,
		elem: elem,
	}
	if v, find := tw.timersManager.Load(key); find {
		oldv := v.([]*timersManagerInfo)
		oldv = append(oldv, newTimerInfo)
		tw.timersManager.Store(key, oldv)
	} else {
		newv := make([]*timersManagerInfo, 0)
		newv = append(newv, newTimerInfo)
		tw.timersManager.Store(key, newv)
	}
}

func (tw *Wheel) del(key interface{}) {
	// 全部定时器都删除
	if v, find := tw.timersManager.Load(key); find {
		oldv := v.([]*timersManagerInfo)
		for _, v := range oldv {
			tw.slots[v.slot].del(v.elem)
		}
		tw.timersManager.Delete(key)
	}
}

func (tw *Wheel) deliverTask(t *Task) {
	tw.taskDeliverChan <- t
}
