package instruments

import (
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/metric"
)

const domainMetricPrefix = "Domain."

type domainInstrument struct {
	bbs bbs.MetricsBBS
}

func NewDomainInstrument(metricsBbs bbs.MetricsBBS) Instrument {
	return &domainInstrument{bbs: metricsBbs}
}

func (t *domainInstrument) Send() {
	// error intentionally dropped; report an empty set in the case of an error
	domains, _ := t.bbs.Domains()

	for _, domain := range domains {
		metric.Metric(domainMetricPrefix + domain).Send(1)
	}
}
