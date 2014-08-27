package instruments

import (
	"github.com/cloudfoundry-incubator/metricz/instrumentation"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
)

type freshnessInstrument struct {
	bbs    bbs.MetricsBBS
	domain string
}

func NewFreshnessInstrument(metricsBbs bbs.MetricsBBS, domain string) instrumentation.Instrumentable {
	return &freshnessInstrument{bbs: metricsBbs, domain: domain}
}

func (t *freshnessInstrument) Emit() instrumentation.Context {
	// error intentionally dropped; report an empty set in the case of an error
	freshDomains, _ := t.bbs.GetAllFreshness()

	var metrics []instrumentation.Metric
	for _, domain := range freshDomains {
		metrics = append(metrics, instrumentation.Metric{
			Name:  domain,
			Value: float64(1),
		})
	}

	return instrumentation.Context{
		Name:    "Freshness",
		Metrics: metrics,
	}
}
