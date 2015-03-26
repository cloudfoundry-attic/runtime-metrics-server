package main

import (
	"flag"
	"os"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/cf-debug-server"
	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/cf_http"
	"github.com/cloudfoundry-incubator/consuladapter"
	"github.com/cloudfoundry-incubator/runtime-metrics-server/metrics"
	Bbs "github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs/lock_bbs"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/nu7hatch/gouuid"
	"github.com/pivotal-golang/clock"
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

var consulCluster = flag.String(
	"consulCluster",
	"",
	"comma-separated list of consul server addresses (ip:port)",
)

var consulScheme = flag.String(
	"consulScheme",
	"http",
	"protocol scheme for communication with consul servers",
)

var consulDatacenter = flag.String(
	"consulDatacenter",
	"dc1",
	"consul datacenter",
)

var lockTTL = flag.Duration(
	"lockTTL",
	lock_bbs.LockTTL,
	"TTL for service lock",
)

var heartbeatRetryInterval = flag.Duration(
	"heartbeatRetryInterval",
	lock_bbs.RetryInterval,
	"interval to wait before retrying a failed lock acquisition",
)

var dropsondeOrigin = flag.String(
	"dropsondeOrigin",
	"runtime_metrics_server",
	"Origin identifier for dropsonde-emitted metrics.",
)

var dropsondeDestination = flag.String(
	"dropsondeDestination",
	"localhost:3457",
	"Destination for dropsonde-emitted metrics.",
)

var communicationTimeout = flag.Duration(
	"communicationTimeout",
	10*time.Second,
	"Timeout applied to all HTTP requests.",
)

func main() {
	cf_debug_server.AddFlags(flag.CommandLine)
	cf_lager.AddFlags(flag.CommandLine)
	flag.Parse()

	cf_http.Initialize(*communicationTimeout)

	logger, reconfigurableSink := cf_lager.New("runtime-metrics-server")
	initializeDropsonde(logger)
	metricsBBS := initializeMetricsBBS(logger, *etcdCluster, *consulCluster, *consulScheme, *consulDatacenter)

	uuid, err := uuid.NewV4()
	if err != nil {
		logger.Fatal("Couldn't generate uuid", err)
	}
	heartbeater := metricsBBS.NewRuntimeMetricsLock(uuid.String(), *lockTTL, *heartbeatRetryInterval)

	notifier := metrics.PeriodicMetronNotifier{
		Interval:    *reportInterval,
		MetricsBBS:  metricsBBS,
		ETCDCluster: strings.Split(*etcdCluster, ","),
		Logger:      logger,
		Clock:       clock.NewClock(),
	}

	members := grouper.Members{
		{"heartbeater", heartbeater},
		{"metrics", notifier},
	}

	if dbgAddr := cf_debug_server.DebugAddress(flag.CommandLine); dbgAddr != "" {
		members = append(grouper.Members{
			{"debug-server", cf_debug_server.Runner(dbgAddr, reconfigurableSink)},
		}, members...)
	}

	group := grouper.NewOrdered(os.Interrupt, members)

	process := ifrit.Invoke(sigmon.New(group))

	logger.Info("started")

	err = <-process.Wait()
	if err != nil {
		logger.Fatal("failed", err)
	} else {
		logger.Info("exited")
	}
}

func initializeDropsonde(logger lager.Logger) {
	err := dropsonde.Initialize(*dropsondeDestination, *dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeMetricsBBS(logger lager.Logger, etcdCluster, consulCluster, scheme, datacenter string) Bbs.MetricsBBS {
	etcdAdapter := etcdstoreadapter.NewETCDStoreAdapter(
		strings.Split(etcdCluster, ","),
		workpool.NewWorkPool(10),
	)

	if err := etcdAdapter.Connect(); err != nil {
		logger.Fatal("failed-to-connect-to-etcd", err)
	}

	consulAdapter, err := consuladapter.NewAdapter(
		strings.Split(consulCluster, ","),
		scheme,
	)
	if err != nil {
		logger.Fatal("failed-building-consul-adapter", err)
	}

	return Bbs.NewMetricsBBS(etcdAdapter, consulAdapter, clock.NewClock(), logger)
}
