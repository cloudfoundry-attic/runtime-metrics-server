package instruments

import (
	"github.com/cloudfoundry-incubator/metricz/instrumentation"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
)

type serviceRegistryInstrument struct {
	bbs bbs.MetricsBBS
}

func NewServiceRegistryInstrument(metricsBbs bbs.MetricsBBS) instrumentation.Instrumentable {
	return &serviceRegistryInstrument{bbs: metricsBbs}
}

func (t *serviceRegistryInstrument) Emit() instrumentation.Context {

	var numExecutors int
	registrations, err := t.bbs.GetServiceRegistrations()
	if err != nil {
		numExecutors = -1
	} else {
		numExecutors = len(registrations.ExecutorRegistrations())
	}

	return instrumentation.Context{
		Name: "ServiceRegistrations",
		Metrics: []instrumentation.Metric{
			{
				Name:  "Executor",
				Value: numExecutors,
			},
		},
	}
}
