package time_heap

import "time"

type ExpireType int

const (
	ExpireOnlyOne ExpireType = 1 + iota
	ExpireEvery
)

type timer struct {
	// 唯一id
	id int
	// 类型
	exType ExpireType
	// 间隔
	dura time.Duration
	// 到时时间
	ts time.Time
	// 到时任务
	t *Task
}

func newTimer(et ExpireType, dura time.Duration, t *Task) *timer {
	next := time.Now().Add(dura)
	return &timer{
		exType: et,
		dura:   dura,
		ts:     next,
		t:      t,
	}
}
