package time_wheel

import "container/list"

type slot struct {
	num   int32
	slots *list.List
}

func NewSlot() *slot {
	return &slot{
		slots: list.New(),
	}
}

// 槽上添加定时器，根据超时时间做插入排序
func (s *slot) add(t *timer) *list.Element {
	tmp := s.slots.Front()
	var elem *list.Element
	for tmp != nil {
		if tmp.Value.(*timer).timeoutTs > t.timeoutTs {
			elem = s.slots.InsertBefore(t, tmp)
			break
		}
		tmp = tmp.Next()
	}
	if tmp == nil {
		elem = s.slots.PushBack(t)
	}
	s.num++
	return elem
}

func (s *slot) del(t *list.Element) {
	if t != nil {
		s.slots.Remove(t)
		s.num--
	}
}

// 查看头定时器
func (s *slot) front() *timer {
	front := s.slots.Front()
	if front != nil {
		return front.Value.(*timer)
	}
	return nil
}

// 出队列头定时器
func (s *slot) pop() *timer {
	front := s.slots.Front()
	if front != nil {
		s.slots.Remove(front)
		s.num--
	}
	return front.Value.(*timer)
}
