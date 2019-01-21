package statem

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

const (
	SimpleOpenCloseState_Open uint32 = 1 + iota
	SimpleOpenCloseState_Close
)

type KVTimeoutTestValue struct {
	TimeoutType uint32
	Value       interface{}
}

type SimpleOpenClose struct {
	curState uint32
	sm       *Machine
	lock     *sync.RWMutex
}

// 这里自己封装一个状态机状态获取供外部调用
func (soc *SimpleOpenClose) State() uint32 {
	soc.lock.RLock()
	defer soc.lock.RUnlock()
	return soc.curState
}

func TestSimpleOpenClose(t *testing.T) {
	soc := &SimpleOpenClose{
		sm:   NewStateMachine(),
		lock: &sync.RWMutex{},
	}
	states := []*State{
		{Current: SimpleOpenCloseState_Close, Next: SimpleOpenCloseState_Open,
			Timeout:       0, // 关闭状态永远不超时，只有主动打开开关才切状态
			EntryCallback: soc.enterClose, ExitCallback: soc.exitClose},
		{Current: SimpleOpenCloseState_Open, Next: SimpleOpenCloseState_Close,
			Timeout:       10, // 打开状态如果10s没有操作就自动切换到下个状态
			EntryCallback: soc.enterOpen, ExitCallback: soc.exitOpen},
	}

	// 初试化状态机为关闭状态
	err := InitStateMachine(soc.sm, SimpleOpenCloseState_Close, states)
	if err != nil {
		panic(err)
	}

	// 打开开关
	err = soc.sm.GotoNext()
	if err != nil {
		t.Fatal(err)
	}

	// 开始给状态机超时tick
	ticker := time.NewTicker(2 * time.Second)
	for {
		timeNow := time.Now()
		select {
		case <-ticker.C:
			soc.sm.Tick(timeNow.Unix())
		}
		fmt.Printf("tick .. \n")
	}
}

func (oc *SimpleOpenClose) enterClose(s *CurState) {
	oc.lock.Lock()
	oc.curState = s.state.Current
	oc.lock.Unlock()

	if s.IsTimeoutExit {
		fmt.Printf("timeout, and auto enter close state! custom data:%v\n", s.CustomData)
	} else {
		fmt.Printf("enter close, custom data:%v\n", s.CustomData)
	}
	oc.sm.curState.CustomData = "test custom data"
}

func (oc *SimpleOpenClose) enterOpen(s *CurState) {
	oc.lock.Lock()
	oc.curState = s.state.Current
	oc.lock.Unlock()

	fmt.Printf("enter open, custom data:%v\n", s.CustomData)
	// 开关进入打开状态就添加两个kv自定义超时器，如果超时之前没有被删掉的话表示触发了这个自定义动作
	err := oc.sm.InsertKeyValueTimeout(&KVTimeout{
		Key:     fmt.Sprintf("test_key:%v", 123),
		Value:   &KVTimeoutTestValue{TimeoutType: 2, Value: 123},
		Timeout: 2,
		Callback: func(key, value interface{}, timeout int32) {
			fmt.Printf("key value timeout, key:%v, type:%v, value:%v, timeout:%v\n",
				key.(string), value.(*KVTimeoutTestValue).TimeoutType, value.(*KVTimeoutTestValue).Value, timeout)
		},
	})
	if err != nil {
		panic(err)
	}
	err = oc.sm.InsertKeyValueTimeout(&KVTimeout{
		Key:     123,
		Value:   "test_value",
		Timeout: 3,
		Callback: func(key, value interface{}, timeout int32) {
			fmt.Printf("key value timeout, key:%v, value:%v, timeout:%v\n", key, value, timeout)
		},
	})
	if err != nil {
		panic(err)
	}
}
func (oc *SimpleOpenClose) exitClose(s *CurState) {
	fmt.Printf("exit close\n")
}
func (oc *SimpleOpenClose) exitOpen(s *CurState) {
	fmt.Printf("exit open, custom data:%v\n", s.CustomData)
	s.CustomData = nil
}
