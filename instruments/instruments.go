package instruments

import (
	"github.com/cloudfoundry-incubator/metricz/instrumentation"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
)

type TaskInstrument struct {
	bbs bbs.MetricsBBS
}

func NewTaskInstrument(metricsBbs bbs.MetricsBBS) *TaskInstrument {
	return &TaskInstrument{bbs: metricsBbs}
}

func (t *TaskInstrument) Emit() instrumentation.Context {
	pendingCount := 0
	claimedCount := 0
	runningCount := 0
	completedCount := 0
	resolvingCount := 0

	allRunOnces, err := t.bbs.GetAllRunOnces()

	if err == nil {
		for _, runOnce := range allRunOnces {
			switch runOnce.State {
			case models.RunOnceStatePending:
				pendingCount++
			case models.RunOnceStateClaimed:
				claimedCount++
			case models.RunOnceStateRunning:
				runningCount++
			case models.RunOnceStateCompleted:
				completedCount++
			case models.RunOnceStateResolving:
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
