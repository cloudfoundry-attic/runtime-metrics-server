package instruments

import (
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/metric"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
)

const serviceRegistrationsMetricPrefix = "ServiceRegistrations"

var serviceNames = []string{
	models.ExecutorServiceName,
	models.FileServerServiceName,
}

type serviceRegistryInstrument struct {
	bbs bbs.MetricsBBS
}

func NewServiceRegistryInstrument(metricsBbs bbs.MetricsBBS) Instrument {
	return &serviceRegistryInstrument{bbs: metricsBbs}
}

func (t *serviceRegistryInstrument) Send() {
	registrations, err := t.bbs.GetServiceRegistrations()
	if err != nil {
		for _, serviceName := range serviceNames {
			metric.Metric(serviceRegistrationsMetricPrefix + serviceName).Send(-1)
		}
	} else {
		for _, serviceName := range serviceNames {
			registrations := len(registrations.FilterByName(serviceName))
			metric.Metric(serviceRegistrationsMetricPrefix + serviceName).Send(registrations)
		}
	}
}
