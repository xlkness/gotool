package time_wheel

import (
	"fmt"
	"testing"
	"time"
)

func TestWheel(t *testing.T) {
	tw := NewAndRun(10, time.Second, 10)
	for i := 10; i < 20; i++ {
		tw.Add(12345+i, time.Second*time.Duration(i), &Task{F: func(a interface{}) { fmt.Printf("hell %v\n", a) }, A: 12345 + i})
	}

	tw.Del(12345 + 10)
	tw.Add(12345+10, time.Second*time.Duration(11), &Task{F: func(a interface{}) { fmt.Printf("hell %v\n", a) }, A: 87956456465})
	tw.Add(12345+10, time.Second*time.Duration(11), &Task{F: func(a interface{}) { fmt.Printf("hell %v\n", a) }, A: 87956456466})

	for {
		time.Sleep(time.Second)
		tw.Add(12345+10, time.Second*time.Duration(11), &Task{F: func(a interface{}) { fmt.Printf("hell %v\n", a) }, A: 1411111111})
		i := 0
		for {
			tw.Del(12345 + 10)
			time.Sleep(time.Second)
			i++
			if i > 30 {
				tw.Stop()
			}
		}
	}
}
