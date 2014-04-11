package main

import (
	"flag"
	"log"
	"strings"

	"github.com/cloudfoundry-incubator/runtime-metrics-server/metrics_server"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	steno "github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"github.com/cloudfoundry/yagnats"
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
		log.Fatalf("Error connecting to NATS: %s\n", err)
	}

	config := metrics_server.Config{
		Port:     uint32(*port),
		Username: *username,
		Password: *password,
		Index:    *index,
	}

	etcdAdapter := etcdstoreadapter.NewETCDStoreAdapter(
		strings.Split(*etcdCluster, ","),
		workerpool.NewWorkerPool(10),
	)

	if err := etcdAdapter.Connect(); err != nil {
		log.Fatalf("Error connecting to etcd: %s\n", err)
	}

	bbs := bbs.New(etcdAdapter, timeprovider.NewTimeProvider())

	server := metrics_server.New(
		natsClient,
		bbs,
		steno.NewLogger("runtime-metrics-server"),
		config,
	)

	log.Fatalln(server.Listen())
}
