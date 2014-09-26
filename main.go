package main

import (
	"flag"
	"os"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/cf-debug-server"
	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/runtime-metrics-server/metrics"
	Bbs "github.com/cloudfoundry-incubator/runtime-schema/bbs"
	_ "github.com/cloudfoundry/dropsonde/autowire"
	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

var etcdCluster = flag.String(
	"etcdCluster",
	"http://127.0.0.1:4001",
	"comma-separated list of etcd addresses (http://ip:port)",
)

var reportInterval = flag.Duration(
	"reportInterval",
	time.Minute,
	"interval on which to report metrics",
)

func main() {
	flag.Parse()

	logger := cf_lager.New("runtime-metrics-server")
	metricsBBS := initializeMetricsBBS(logger)

	cf_debug_server.Run()

	notifier := metrics.PeriodicMetronNotifier{
		Interval:   *reportInterval,
		MetricsBBS: metricsBBS,
	}

	group := grouper.NewOrdered(os.Interrupt, grouper.Members{
		{"metrics", notifier},
	})

	process := ifrit.Envoke(sigmon.New(group))

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
