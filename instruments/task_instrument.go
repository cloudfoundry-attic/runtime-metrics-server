package instruments

import (
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/metric"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
)

const (
	pendingTasks   = metric.Metric("TasksPending")
	runningTasks   = metric.Metric("TasksRunning")
	completedTasks = metric.Metric("TasksCompleted")
	resolvingTasks = metric.Metric("TasksResolving")
)

type taskInstrument struct {
	bbs bbs.MetricsBBS
}

func NewTaskInstrument(metricsBbs bbs.MetricsBBS) Instrument {
	return &taskInstrument{bbs: metricsBbs}
}

func (t *taskInstrument) Send() {
	pendingCount := 0
	runningCount := 0
	completedCount := 0
	resolvingCount := 0

	allTasks, err := t.bbs.Tasks()

	if err == nil {
		for _, task := range allTasks {
			switch task.State {
			case models.TaskStatePending:
				pendingCount++
			case models.TaskStateRunning:
				runningCount++
			case models.TaskStateCompleted:
				completedCount++
			case models.TaskStateResolving:
				resolvingCount++
			}
		}
	} else {
		pendingCount = -1
		runningCount = -1
		completedCount = -1
		resolvingCount = -1
	}

	pendingTasks.Send(pendingCount)
	runningTasks.Send(runningCount)
	completedTasks.Send(completedCount)
	resolvingTasks.Send(resolvingCount)
}
