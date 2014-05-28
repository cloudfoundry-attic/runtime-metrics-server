package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/cloudfoundry-incubator/runtime-metrics-server/metrics_server"
	Bbs "github.com/cloudfoundry-incubator/runtime-schema/bbs"
	steno "github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"github.com/cloudfoundry/yagnats"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/sigmon"
)

var etcdCluster = flag.String(
	"etcdCluster",
	"http://127.0.0.1:4001",
	"comma-separated list of etcd addresses (http://ip:port)",
)

var index = flag.Uint(
	"index",
	0,
	"index of the etcd job",
)

var port = flag.Uint(
	"port",
	5678,
	"port to listen on",
)

var username = flag.String(
	"username",
	"",
	"basic auth username",
)

var password = flag.String(
	"password",
	"",
	"basic auth password",
)

var natsAddresses = flag.String(
	"natsAddresses",
	"127.0.0.1:4222",
	"comma-separated list of NATS addresses (ip:port)",
)

var natsUsername = flag.String(
	"natsUsername",
	"nats",
	"Username to connect to nats",
)

var natsPassword = flag.String(
	"natsPassword",
	"nats",
	"Password for nats user",
)

var syslogName = flag.String(
	"syslogName",
	"",
	"syslog program name",
)

func main() {
	flag.Parse()

	logger := initializeLogger()
	natsClient := initializeNatsClient(logger)
	metricsBBS := initializeMetricsBBS(logger)

	config := metrics_server.Config{
		Port:     uint32(*port),
		Username: *username,
		Password: *password,
		Index:    *index,
	}

	server := ifrit.Envoke(metrics_server.New(
		natsClient,
		metricsBBS,
		logger,
		config,
	))

	monitor := ifrit.Envoke(sigmon.New(server))

	err := <-monitor.Wait()
	if err != nil {
		log.Fatal("runtime-metrics-server exited with error: %s", err)
	}
}

func initializeLogger() *steno.Logger {
	stenoConfig := steno.Config{
		Level: steno.LOG_INFO,
		Sinks: []steno.Sink{steno.NewIOSink(os.Stdout)},
	}

	if *syslogName != "" {
		stenoConfig.Sinks = append(stenoConfig.Sinks, steno.NewSyslogSink(*syslogName))
	}

	steno.Init(&stenoConfig)

	return steno.NewLogger("runtime-metrics-server")
}

func initializeNatsClient(logger *steno.Logger) *yagnats.Client {
	natsClient := yagnats.NewClient()

	natsMembers := []yagnats.ConnectionProvider{}

	for _, addr := range strings.Split(*natsAddresses, ",") {
		natsMembers = append(
			natsMembers,
			&yagnats.ConnectionInfo{addr, *natsUsername, *natsPassword},
		)
	}

	err := natsClient.Connect(&yagnats.ConnectionCluster{
		Members: natsMembers,
	})

	if err != nil {
		logger.Fatalf("Error connecting to NATS: %s\n", err)
	}

	return natsClient
}

func initializeMetricsBBS(logger *steno.Logger) Bbs.MetricsBBS {
	etcdAdapter := etcdstoreadapter.NewETCDStoreAdapter(
		strings.Split(*etcdCluster, ","),
		workerpool.NewWorkerPool(10),
	)

	if err := etcdAdapter.Connect(); err != nil {
		logger.Fatalf("Error connecting to etcd: %s\n", err)
	}

	return Bbs.NewMetricsBBS(etcdAdapter, timeprovider.NewTimeProvider(), logger)
}
