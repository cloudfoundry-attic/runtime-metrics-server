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
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	Bbs "github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs/shared"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/pivotal-golang/clock"
)

var _ = Describe("Runtime Metrics Server", func() {
	var (
		bbs *Bbs.BBS

		process ifrit.Process

		metricsServerLockName  = "runtime_metrics_lock"
		lockTTL                time.Duration
		heartbeatRetryInterval time.Duration
		reportInterval         time.Duration
		testMetricsListener    net.PacketConn
		testMetricsChan        chan *events.ValueMetric
	)

	startMetricsServer := func(check bool) {
		cmd := exec.Command(metricsServerPath,
			"-etcdCluster", strings.Join(etcdRunner.NodeURLS(), ","),
			"-reportInterval", reportInterval.String(),
			"-consulCluster", consulRunner.ConsulCluster(),
			"-lockTTL", lockTTL.String(),
			"-heartbeatRetryInterval", heartbeatRetryInterval.String(),
			"-dropsondeOrigin", "test-metrics-server",
			"-dropsondeDestination", testMetricsListener.LocalAddr().String(),
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
		bbs = Bbs.NewBBS(etcdClient, consulAdapter, clock.NewClock(), lagertest.NewTestLogger("test"))

		lockTTL = structs.SessionTTLMin
		heartbeatRetryInterval = 100 * time.Millisecond
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
				Ω(err).ToNot(HaveOccurred())

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
		BeforeEach(func() {
			_, err := consulAdapter.AcquireAndMaintainLock(
				shared.LockSchemaPath(metricsServerLockName),
				[]byte("something-else"),
				structs.SessionTTLMin,
				nil,
			)
			Ω(err).ShouldNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			startMetricsServer(false)
		})

		It("does not emit any metrics", func() {
			Consistently(testMetricsChan).ShouldNot(Receive())
		})

		Context("when the lock becomes available", func() {
			BeforeEach(func() {
				err := consulAdapter.ReleaseAndDeleteLock(shared.LockSchemaPath(metricsServerLockName))
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("starts emitting metrics", func() {
				Eventually(testMetricsChan).Should(Receive())
			})
		})
	})
})
