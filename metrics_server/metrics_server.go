package metrics_server

import (
	"os"

	"github.com/cloudfoundry-incubator/metricz"
	"github.com/cloudfoundry-incubator/metricz/collector_registrar"
	"github.com/cloudfoundry-incubator/metricz/instrumentation"
	"github.com/cloudfoundry-incubator/runtime-metrics-server/health_check"
	"github.com/cloudfoundry-incubator/runtime-metrics-server/instruments"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/yagnats"
)

type Config struct {
	Port     uint32
	Username string
	Password string
	Index    uint
}

type MetricsServer struct {
	natsClient yagnats.NATSClient
	bbs        bbs.MetricsBBS
	logger     *gosteno.Logger
	config     Config
	component  metricz.Component
}

func New(
	natsClient yagnats.NATSClient,
	bbs bbs.MetricsBBS,
	logger *gosteno.Logger,
	config Config,
) *MetricsServer {
	return &MetricsServer{
		natsClient: natsClient,
		bbs:        bbs,
		logger:     logger,
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
		},
	)

	err = registrar.RegisterWithCollector(server.component)
	if err != nil {
		return err
	}

	close(ready)

	serverStopped := make(chan error)
	go func() {
		serverStopped <- server.component.StartMonitoringEndpoints()
	}()

	select {
	case <-signals:
		server.component.StopMonitoringEndpoints()
		return nil
	case err := <-serverStopped:
		return err
	}
}

/*
func (server *MetricsServer) Listen() error {
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
		},
	)

	err = registrar.RegisterWithCollector(server.component)
	if err != nil {
		return err
	}

	server.component.StartMonitoringEndpoints()
}

func (server *MetricsServer) Stop() {
	server.component.StopMonitoringEndpoints()
}
*/
