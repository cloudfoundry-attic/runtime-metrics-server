package main_test

import (
	"testing"

	"github.com/cloudfoundry-incubator/consuladapter"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var metricsServerPath string

var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdClient storeadapter.StoreAdapter
var consulScheme string
var consulDatacenter string
var consulRunner *consuladapter.ClusterRunner
var consulSession *consuladapter.Session

func TestBulker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Runtime Metrics Server Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	metricsServer, err := gexec.Build("github.com/cloudfoundry-incubator/runtime-metrics-server/cmd/runtime-metrics-server", "-race")
	Expect(err).NotTo(HaveOccurred())
	return []byte(metricsServer)
}, func(metricsServer []byte) {
	etcdPort := 5001 + GinkgoParallelNode()
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	metricsServerPath = string(metricsServer)
	etcdClient = etcdRunner.Adapter()

	consulScheme = "http"
	consulDatacenter = "dc"
	consulRunner = consuladapter.NewClusterRunner(
		9001+GinkgoParallelNode()*consuladapter.PortOffsetLength,
		1,
		consulScheme,
	)

	etcdRunner.Start()
	consulRunner.Start()
	consulRunner.WaitUntilReady()
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()

	consulRunner.Reset()
	consulSession = consulRunner.NewSession("a-session")
})

var _ = SynchronizedAfterSuite(func() {
	etcdRunner.Stop()
	consulRunner.Stop()
}, func() {
	gexec.CleanupBuildArtifacts()
})
