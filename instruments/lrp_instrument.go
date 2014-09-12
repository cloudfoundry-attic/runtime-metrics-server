package instruments

import (
	"github.com/cloudfoundry-incubator/metricz/instrumentation"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
)

type lrpInstrument struct {
	bbs bbs.MetricsBBS
}

func NewLRPInstrument(metricsBbs bbs.MetricsBBS) instrumentation.Instrumentable {
	return &lrpInstrument{bbs: metricsBbs}
}

func (t *lrpInstrument) Emit() instrumentation.Context {
	desiredCount := 0
	runningCount := 0
	startingCount := 0

	allDesiredLRPs, err := t.bbs.GetAllDesiredLRPs()
	if err == nil {
		for _, lrp := range allDesiredLRPs {
			desiredCount += lrp.Instances
		}
	} else {
		desiredCount = -1
	}

	allActualLRPs, err := t.bbs.GetAllActualLRPs()
	if err == nil {
		for _, lrp := range allActualLRPs {
			switch lrp.State {
			case models.ActualLRPStateStarting:
				startingCount++
			case models.ActualLRPStateRunning:
				runningCount++
			}
		}
	} else {
		startingCount = -1
		runningCount = -1
	}

	return instrumentation.Context{
		Name: "LRPs",
		Metrics: []instrumentation.Metric{
			{
				Name:  "Desired",
				Value: desiredCount,
			},
			{
				Name:  "Starting",
				Value: startingCount,
			},
			{
				Name:  "Running",
				Value: runningCount,
			},
		},
	}
}
