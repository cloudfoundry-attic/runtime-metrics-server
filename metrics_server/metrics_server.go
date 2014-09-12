package metrics_server

import (
	"os"

	"github.com/cloudfoundry-incubator/metricz"
	"github.com/cloudfoundry-incubator/metricz/collector_registrar"
	"github.com/cloudfoundry-incubator/metricz/instrumentation"
	"github.com/cloudfoundry-incubator/runtime-metrics-server/health_check"
	"github.com/cloudfoundry-incubator/runtime-metrics-server/instruments"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry/yagnats"
	"github.com/pivotal-golang/lager"
)

type Config struct {
	Port     uint32
	Username string
	Password string
	Index    uint
	Domain   string
}

type MetricsServer struct {
	natsClient yagnats.NATSClient
	bbs        bbs.MetricsBBS
	logger     lager.Logger
	config     Config
	component  metricz.Component
}

func New(
	natsClient yagnats.NATSClient,
	bbs bbs.MetricsBBS,
	logger lager.Logger,
	config Config,
) *MetricsServer {
	serverLogger := logger.Session("metrics-server")
	return &MetricsServer{
		natsClient: natsClient,
		bbs:        bbs,
		logger:     serverLogger,
		config:     config,
	}
}

func (server *MetricsServer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	registrar := collector_registrar.New(server.natsClient)

	var err error
	server.component, err = metricz.NewComponent(
		server.logger,
		"runtime",
		server.config.Index,
		health_check.New(),
		server.config.Port,
		[]string{server.config.Username, server.config.Password},
		[]instrumentation.Instrumentable{
			instruments.NewTaskInstrument(server.bbs),
			instruments.NewServiceRegistryInstrument(server.bbs),
			instruments.NewFreshnessInstrument(server.bbs, server.config.Domain),
			instruments.NewLRPInstrument(server.bbs),
		},
	)

	err = registrar.RegisterWithCollector(server.component)
	if err != nil {
		return err
	}

	serverStopped := make(chan error)
	go func() {
		serverStopped <- server.component.StartMonitoringEndpoints()
	}()

	close(ready)

	select {
	case signal := <-signals:
		server.logger.Info("stopping-on-signal", lager.Data{"signal": signal})
		server.component.StopMonitoringEndpoints()
		return nil
	case err := <-serverStopped:
		server.logger.Error("stopping-on-failure", err)
		return err
	}
}
