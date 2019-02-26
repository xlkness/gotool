package statem

import (
	"fmt"
	"time"
)

type KVTimeout struct {
	Key   interface{}
	Value interface{}
	// 超时的时间，秒
	Timeout int32
	// 超时的回调函数
	Callback func(key interface{}, value interface{}, timeout int32)
	// 超时的时间戳，为Tick定时精度的整数倍
	timeoutTs int64
}

type State struct {
	// 当前状态
	Current uint32
	// 超时秒数，为0表示一直不超时，调用方
	// 手动切换状态，为Tick定时精度的整数倍
	Timeout int32
	// 超时执行的回调函数，为nil表示无
	ExitCallback func(*CurState)
	// 切换到当前状态执行的回调函数，为nil表示无
	EntryCallback func(*CurState)
	// 超时切换的下一个状态，为0表示无
	Next uint32
}

type CurState struct {
	state *State
	// 是否超时离开状态
	IsTimeoutExit bool
	// 当前状态维护的自定义数据，如果状态切换回调不改变，则这个值会一直保持一样
	CustomData interface{}
}

type Machine struct {
	// 当前状态
	// todo 当前状态机只能单线程使用，如果多线程要获取当前状态，要在进入状态的回调自己封装状态标记
	curState *CurState
	// 当前状态超时时间戳，<0表示永久维持当前状态
	// 例如房间状态机中，准备时间只能30s，到时则强制进入下个状态
	curStateTimeout int64
	// 当前状态中用于自定义的超时时间，切换状态会清空
	// 例如房间状态机中，如果某个玩家在准备阶段10s未准备则踢出去
	kvTimeouts map[interface{}]*KVTimeout
	// 状态描述列表
	states map[uint32]*State
}

// 创建新的状态机
// 状态机不支持并发访问
func NewStateMachine() *Machine {
	m := &Machine{}
	return m
}
func InitStateMachine(m *Machine, initState uint32, states []*State) error {

	// valid check
	stateList := make(map[uint32]*State)
	for _, v := range states {
		if _, find := stateList[v.Current]; find {
			return fmt.Errorf("reduplicated state:%v", v.Current)
		}
		stateList[v.Current] = v
	}
	for _, v := range stateList {
		if _, find := stateList[v.Next]; !find {
			if v.Next != 0 {
				return fmt.Errorf("not found state:%v", v.Next)
			}
		}
	}
	// goto init state
	if init, find := stateList[initState]; find {
		m.curState = &CurState{state: init, CustomData: nil}
		if init.Timeout > 0 {
			m.curStateTimeout = addTimeoutSecondFromNow(init.Timeout)
		}
	} else {
		return fmt.Errorf("not found init state:%v", initState)
	}
	m.kvTimeouts = make(map[interface{}]*KVTimeout)
	m.states = stateList
	err := m.gotoState(initState, false)
	return err
}

// 获取当前的状态-可多线程调用
func (m *Machine) State() uint32 {
	return m.curState.state.Current
}

// 获取当前的状态的所有信息-可多线程调用
func (m *Machine) StateAll() *CurState {
	return m.curState
}

// 插入新的kv超时提醒，存在返回错误
func (m *Machine) InsertKeyValueTimeout(kvInfo *KVTimeout) error {
	kvInfo.timeoutTs = addTimeoutSecondFromNow(kvInfo.Timeout)
	if _, find := m.kvTimeouts[kvInfo.Key]; find {
		return fmt.Errorf("already has key-value timeout key:%v", kvInfo.Key)
	}
	m.kvTimeouts[kvInfo.Key] = kvInfo
	return nil
}

// 更新旧的kv超时提醒，不存在则插入
func (m *Machine) UpdateKeyValueTimeout(kvInfo *KVTimeout) {
	if oldV, find := m.kvTimeouts[kvInfo.Key]; find {
		oldV.Value = kvInfo.Value
		oldV.Timeout = kvInfo.Timeout
		oldV.timeoutTs = addTimeoutSecondFromNow(kvInfo.Timeout)
		return
	}
	m.kvTimeouts[kvInfo.Key] = kvInfo
}

// 删除kv超时提醒
func (m *Machine) DelKeyValueTimeout(key interface{}) {
	m.delKeyValueTimeout(key)
}

func (m *Machine) GetKeyValueTimeout(key interface{}) (*KVTimeout, bool) {
	v, f := m.kvTimeouts[key]
	return v, f
}

// 跳下一个状态
func (m *Machine) GotoNext() error {
	return m.gotoNext(false)
}

// 跳指定状态
func (m *Machine) GotoState(state uint32) error {
	return m.gotoState(state, false)
}

// 调用方间隔调用，超时时间为秒
func (m *Machine) Tick(timeNow int64) error {
	// check state timeout
	if timeNow >= m.curStateTimeout && m.curState.state.Next != 0 &&
		m.curState.state.Timeout != 0 {
		if m.curState.state.ExitCallback != nil {
			m.curState.state.ExitCallback(m.curState)
		}
		return m.gotoNext(true)
	}

	// check key value timeout
	for k, v := range m.kvTimeouts {
		if timeNow >= v.timeoutTs {
			v.Callback(k, v.Value, v.Timeout)
			m.delKeyValueTimeout(k)
		}
	}
	return nil
}

func (m *Machine) gotoNext(isTimeout bool) error {
	if m.curState.state.Next != 0 {
		return m.gotoState(m.curState.state.Next, isTimeout)
	}
	return nil
}

func (m *Machine) gotoState(state uint32, isTimeout bool) error {
	if state == 0 {
		return nil
	}
	if state, find := m.states[state]; find {
		m.curState.state = state
		m.curStateTimeout = addTimeoutSecondFromNow(state.Timeout)
		m.curState.IsTimeoutExit = isTimeout
		m.kvTimeouts = make(map[interface{}]*KVTimeout)
		if state.EntryCallback != nil {
			state.EntryCallback(m.curState)
		}
		return nil
	}
	return fmt.Errorf("not found state:%v", state)
}

// 删除kv超时提醒
func (m *Machine) delKeyValueTimeout(key interface{}) {
	delete(m.kvTimeouts, key)
}

func addTimeoutSecondFromNow(sec int32) int64 {
	return time.Now().Add(time.Second * time.Duration(sec)).Unix()
}
