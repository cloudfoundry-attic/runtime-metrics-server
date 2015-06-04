package main_test

import (
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"github.com/cloudfoundry-incubator/consuladapter"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs/shared"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/hashicorp/consul/consul/structs"
)

var _ = Describe("Runtime Metrics Server", func() {
	var (
		process ifrit.Process

		metricsServerLockName = "runtime_metrics_lock"
		lockTTL               time.Duration
		lockRetryInterval     time.Duration
		reportInterval        time.Duration
		testMetricsListener   net.PacketConn
		testMetricsChan       chan *events.ValueMetric
	)

	startMetricsServer := func(check bool) {
		cmd := exec.Command(metricsServerPath,
			"-etcdCluster", strings.Join(etcdRunner.NodeURLS(), ","),
			"-reportInterval", reportInterval.String(),
			"-consulCluster", consulRunner.ConsulCluster(),
			"-lockTTL", lockTTL.String(),
			"-lockRetryInterval", lockRetryInterval.String(),
			"-dropsondeOrigin", "test-metrics-server",
			"-dropsondeDestination", testMetricsListener.LocalAddr().String(),
			"-diegoAPIURL", "http://receptor.bogus.com",
		)

		runner := ginkgomon.New(ginkgomon.Config{
			Name:              "metrics-server",
			AnsiColorCode:     "97m",
			StartCheck:        "runtime-metrics-server.started",
			StartCheckTimeout: 5 * time.Second,
			Command:           cmd,
		})

		if !check {
			runner.StartCheck = ""
		}

		process = ginkgomon.Invoke(runner)
	}

	BeforeEach(func() {
		lockTTL = structs.SessionTTLMin
		lockRetryInterval = 100 * time.Millisecond
		reportInterval = 10 * time.Millisecond

		testMetricsListener, _ = net.ListenPacket("udp", "127.0.0.1:0")
		testMetricsChan = make(chan *events.ValueMetric, 1)
		go func() {
			defer GinkgoRecover()
			for {
				buffer := make([]byte, 1024)
				n, _, err := testMetricsListener.ReadFrom(buffer)
				if err != nil {
					close(testMetricsChan)
					return
				}

				var envelope events.Envelope
				err = proto.Unmarshal(buffer[:n], &envelope)
				Expect(err).NotTo(HaveOccurred())

				if envelope.GetEventType() == events.Envelope_ValueMetric &&
					envelope.ValueMetric.GetName() == "TasksPending" {
					select {
					case testMetricsChan <- envelope.ValueMetric:
					default:
					}
				}
			}
		}()
	})

	AfterEach(func() {
		testMetricsListener.Close()
		Eventually(testMetricsChan).Should(BeClosed())
		process.Signal(os.Interrupt)
		Eventually(process.Wait(), 5).Should(Receive())
	})

	Context("when the metrics server initially does not have the lock", func() {
		var otherSession *consuladapter.Session

		BeforeEach(func() {
			otherSession = consulRunner.NewSession("other-session")
			err := otherSession.AcquireLock(shared.LockSchemaPath(metricsServerLockName), []byte("something-else"))
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			startMetricsServer(false)
		})

		It("does not emit any metrics", func() {
			Consistently(testMetricsChan, 2*lockRetryInterval).ShouldNot(Receive())
		})

		Context("when the lock becomes available", func() {
			BeforeEach(func() {
				otherSession.Destroy()
			})

			It("starts emitting metrics", func() {
				Eventually(testMetricsChan).Should(Receive())
			})
		})
	})
})
