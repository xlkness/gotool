package time_wheel

import "time"

type Timer struct {
	// 主键，用来删除定时器
	key interface{}
	// 超时时间间隔
	timeout time.Duration
	// 超时到时时间戳
	timeoutTs int64
	// 超时执行任务
	task *Task
}
