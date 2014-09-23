package metrics

import (
	"os"
	"time"

	"github.com/cloudfoundry-incubator/runtime-metrics-server/instruments"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
)

type PeriodicMetronNotifier struct {
	Interval   time.Duration
	MetricsBBS bbs.MetricsBBS
}

func (notifier PeriodicMetronNotifier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	ticker := time.NewTicker(notifier.Interval)

	tasksInstrument := instruments.NewTaskInstrument(notifier.MetricsBBS)
	lrpsInstrument := instruments.NewLRPInstrument(notifier.MetricsBBS)
	freshnessInstrument := instruments.NewFreshnessInstrument(notifier.MetricsBBS)
	serviceRegistryInstrument := instruments.NewServiceRegistryInstrument(notifier.MetricsBBS)

	for {
		select {
		case <-ticker.C:
			tasksInstrument.Send()
			lrpsInstrument.Send()
			freshnessInstrument.Send()
			serviceRegistryInstrument.Send()

		case <-signals:
			return nil
		}
	}

	return nil
}
