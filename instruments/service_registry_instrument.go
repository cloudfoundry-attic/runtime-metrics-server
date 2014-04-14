package instruments

import (
	"github.com/cloudfoundry-incubator/metricz/instrumentation"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
)

var serviceNames = []string{
	models.ExecutorServiceName,
	models.FileServerServiceName,
}

type serviceRegistryInstrument struct {
	bbs bbs.MetricsBBS
}

func NewServiceRegistryInstrument(metricsBbs bbs.MetricsBBS) instrumentation.Instrumentable {
	return &serviceRegistryInstrument{bbs: metricsBbs}
}

func (t *serviceRegistryInstrument) Emit() instrumentation.Context {
	context := instrumentation.Context{
		Name: "ServiceRegistrations",
	}

	registrations, err := t.bbs.GetServiceRegistrations()
	if err != nil {
		for _, serviceName := range serviceNames {
			context.Metrics = append(context.Metrics, instrumentation.Metric{
				Name:  serviceName,
				Value: -1,
			})
		}
	} else {
		for _, serviceName := range serviceNames {
			context.Metrics = append(context.Metrics, instrumentation.Metric{
				Name:  serviceName,
				Value: len(registrations.FilterByName(serviceName)),
			})
		}
	}

	return context
}
