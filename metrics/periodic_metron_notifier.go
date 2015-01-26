package metrics

import (
	"os"
	"time"

	"github.com/cloudfoundry-incubator/runtime-metrics-server/instruments"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/metric"
	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/pivotal-golang/lager"
)

const metricsReportingDuration = metric.Duration("MetricsReportingDuration")

type PeriodicMetronNotifier struct {
	Interval     time.Duration
	MetricsBBS   bbs.MetricsBBS
	Logger       lager.Logger
	TimeProvider timeprovider.TimeProvider
}

func (notifier PeriodicMetronNotifier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	ticker := notifier.TimeProvider.NewTicker(notifier.Interval)

	close(ready)

	tasksInstrument := instruments.NewTaskInstrument(notifier.Logger, notifier.MetricsBBS)
	lrpsInstrument := instruments.NewLRPInstrument(notifier.MetricsBBS)
	domainInstrument := instruments.NewDomainInstrument(notifier.MetricsBBS)
	serviceRegistryInstrument := instruments.NewServiceRegistryInstrument(notifier.MetricsBBS)

	for {
		select {
		case <-ticker.C():
			startedAt := notifier.TimeProvider.Now()

			tasksInstrument.Send()
			lrpsInstrument.Send()
			domainInstrument.Send()
			serviceRegistryInstrument.Send()

			finishedAt := notifier.TimeProvider.Now()

			metricsReportingDuration.Send(finishedAt.Sub(startedAt))

		case <-signals:
			return nil
		}
	}

	return nil
}
