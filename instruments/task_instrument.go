package instruments

import (
	"github.com/cloudfoundry-incubator/metricz/instrumentation"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
)

type taskInstrument struct {
	bbs bbs.MetricsBBS
}

func NewTaskInstrument(metricsBbs bbs.MetricsBBS) instrumentation.Instrumentable {
	return &taskInstrument{bbs: metricsBbs}
}

func (t *taskInstrument) Emit() instrumentation.Context {
	pendingCount := 0
	claimedCount := 0
	runningCount := 0
	completedCount := 0
	resolvingCount := 0

	allTasks, err := t.bbs.GetAllTasks()

	if err == nil {
		for _, runOnce := range allTasks {
			switch runOnce.State {
			case models.TaskStatePending:
				pendingCount++
			case models.TaskStateClaimed:
				claimedCount++
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
		claimedCount = -1
		runningCount = -1
		completedCount = -1
		resolvingCount = -1
	}

	return instrumentation.Context{
		Name: "Tasks",
		Metrics: []instrumentation.Metric{
			{
				Name:  "Pending",
				Value: pendingCount,
			},
			{
				Name:  "Claimed",
				Value: claimedCount,
			},
			{
				Name:  "Running",
				Value: runningCount,
			},
			{
				Name:  "Completed",
				Value: completedCount,
			},
			{
				Name:  "Resolving",
				Value: resolvingCount,
			},
		},
	}
}
