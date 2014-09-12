package instruments

import (
	"github.com/cloudfoundry-incubator/metricz/instrumentation"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
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

	allDesiredLRPs, err := t.bbs.GetAllDesiredLRPs()
	if err == nil {
		for _, lrp := range allDesiredLRPs {
			desiredCount += lrp.Instances
		}
	} else {
		desiredCount = -1
	}

	runningActualLRPs, err := t.bbs.GetRunningActualLRPs()
	if err == nil {
		runningCount = len(runningActualLRPs)
	} else {
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
				Name:  "Running",
				Value: runningCount,
			},
		},
	}
}
