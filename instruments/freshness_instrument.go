package instruments

import (
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/metric"
)

const freshnessMetricPrefix = "Freshness."

type freshnessInstrument struct {
	bbs bbs.MetricsBBS
}

func NewFreshnessInstrument(metricsBbs bbs.MetricsBBS) Instrument {
	return &freshnessInstrument{bbs: metricsBbs}
}

func (t *freshnessInstrument) Send() {
	// error intentionally dropped; report an empty set in the case of an error
	freshnesses, _ := t.bbs.Freshnesses()

	for _, freshness := range freshnesses {
		metric.Metric(freshnessMetricPrefix + freshness.Domain).Send(1)
	}
}
