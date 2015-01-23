package instruments

import (
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/metric"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
)

const (
	desiredLRPs         = metric.Metric("LRPsDesired")
	startingLRPs        = metric.Metric("LRPsStarting")
	runningLRPs         = metric.Metric("LRPsRunning")
	crashedActualLRPs   = metric.Metric("CrashedActualLRPs")
	crashingDesiredLRPs = metric.Metric("CrashingDesiredLRPs")
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
	crashedCount := 0

	allDesiredLRPs, err := t.bbs.DesiredLRPs()
	if err == nil {
		for _, lrp := range allDesiredLRPs {
			desiredCount += lrp.Instances
		}
	} else {
		desiredCount = -1
	}

	crashingDesireds := map[string]struct{}{}

	allActualLRPs, err := t.bbs.ActualLRPs()
	if err == nil {
		for _, lrp := range allActualLRPs {
			switch lrp.State {
			case models.ActualLRPStateClaimed:
				startingCount++
			case models.ActualLRPStateRunning:
				runningCount++
			case models.ActualLRPStateCrashed:
				crashingDesireds[lrp.ProcessGuid] = struct{}{}
				crashedCount++
			}
		}
	} else {
		startingCount = -1
		runningCount = -1
	}

	desiredLRPs.Send(desiredCount)
	startingLRPs.Send(startingCount)
	runningLRPs.Send(runningCount)
	crashedActualLRPs.Send(crashedCount)
	crashingDesiredLRPs.Send(len(crashingDesireds))
}
