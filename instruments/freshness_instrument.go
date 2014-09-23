package instruments

import (
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/metric"
)

const freshnessMetricPrefix = "freshness-"

type freshnessInstrument struct {
	bbs bbs.MetricsBBS
}

func NewFreshnessInstrument(metricsBbs bbs.MetricsBBS) Instrument {
	return &freshnessInstrument{bbs: metricsBbs}
}

func (t *freshnessInstrument) Send() {
	// error intentionally dropped; report an empty set in the case of an error
	freshDomains, _ := t.bbs.GetAllFreshness()

	for _, domain := range freshDomains {
		metric.Metric(freshnessMetricPrefix + domain).Send(1)
	}
}
