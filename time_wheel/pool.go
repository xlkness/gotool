package time_wheel

import (
	"context"
)

type Task struct {
	F func(interface{})
	A interface{}
}

func startWorkerPool(num int32, taskChan <-chan *Task, ctx context.Context) {
	for i := 0; i < int(num); i++ {
		go func(_ int) {
			for {
				select {
				case task, ok := <-taskChan:
					if !ok {
						return
					}
					//fmt.Printf("exec by:%v\n", no)
					task.F(task.A)
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}
}
