package instruments

import (
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/metric"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
)

const (
	desiredLRPs  = metric.Metric("LRPsDesired")
	startingLRPs = metric.Metric("LRPsStarting")
	runningLRPs  = metric.Metric("LRPsRunning")
)

type lrpInstrument struct {
	bbs bbs.MetricsBBS
}

func NewLRPInstrument(metricsBbs bbs.MetricsBBS) Instrument {
	return &lrpInstrument{bbs: metricsBbs}
}

func (t *lrpInstrument) Send() {
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

	desiredLRPs.Send(desiredCount)
	startingLRPs.Send(startingCount)
	runningLRPs.Send(runningCount)
}
