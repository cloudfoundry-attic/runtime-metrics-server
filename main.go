package main

import (
	"flag"
	"strings"

	"github.com/cloudfoundry-incubator/cf-debug-server"
	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/runtime-metrics-server/metrics_server"
	"github.com/cloudfoundry-incubator/runtime-metrics-server/nats_client"
	Bbs "github.com/cloudfoundry-incubator/runtime-schema/bbs"
	_ "github.com/cloudfoundry/dropsonde/autowire"
	"github.com/cloudfoundry/gunk/group_runner"
	"github.com/cloudfoundry/gunk/natsclientrunner"
	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"github.com/pivotal-golang/lager"
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

func main() {
	flag.Parse()

	logger := cf_lager.New("runtime-metrics-server")
	metricsBBS := initializeMetricsBBS(logger)

	cf_debug_server.Run()

	natsClient := nats_client.New(*natsAddresses, *natsUsername, *natsPassword)
	natsClientRunner := natsclientrunner.New(natsClient, logger)

	metricsServer := metrics_server.New(
		natsClient,
		metricsBBS,
		logger,
		metrics_server.Config{
			Port:     uint32(*port),
			Username: *username,
			Password: *password,
			Index:    *index,
		},
	)

	process := ifrit.Envoke(sigmon.New(group_runner.New([]group_runner.Member{
		{"nats", natsClientRunner},
		{"metrics", metricsServer},
	})))

	logger.Info("started")

	err := <-process.Wait()
	if err != nil {
		logger.Fatal("failed", err)
	} else {
		logger.Info("exited")
	}
}

func initializeMetricsBBS(logger lager.Logger) Bbs.MetricsBBS {
	etcdAdapter := etcdstoreadapter.NewETCDStoreAdapter(
		strings.Split(*etcdCluster, ","),
		workerpool.NewWorkerPool(10),
	)

	if err := etcdAdapter.Connect(); err != nil {
		logger.Fatal("failed-to-connect-to-etcd", err)
	}

	return Bbs.NewMetricsBBS(etcdAdapter, timeprovider.NewTimeProvider(), logger)
}
