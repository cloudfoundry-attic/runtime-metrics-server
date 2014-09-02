package main_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/cloudfoundry-incubator/metricz/collector_registrar"
	"github.com/cloudfoundry/gunk/natsrunner"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/cloudfoundry/yagnats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

func TestRuntimeMetricsServer(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultConsistentlyDuration(time.Second)
	SetDefaultConsistentlyPollingInterval(100 * time.Millisecond)
	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	RunSpecs(t, "RuntimeMetricsServer Suite")
}

func NewMetricServer(binPath string, metricsPort, etcdPort, natsPort int) *ginkgomon.Runner {
	return ginkgomon.New(ginkgomon.Config{
		Name:          "metrics",
		AnsiColorCode: "61m",
		Command: exec.Command(
			binPath,
			"-port", fmt.Sprintf("%d", metricsPort),
			"-etcdCluster", fmt.Sprintf("http://127.0.0.1:%d", etcdPort),
			"-natsAddresses", fmt.Sprintf("127.0.0.1:%d", natsPort),
			"-index", "5",
			"-username", "the-username",
			"-password", "the-password",
		),
	})
}

var _ = Describe("Main", func() {
	var metricsPort int
	var natsPort int
	var etcdPort int
	var nats *natsrunner.NATSRunner
	var etcdRunner *etcdstorerunner.ETCDClusterRunner
	var metricsServerPath string
	var metricsServer *ginkgomon.Runner
	var metricsProcess ifrit.Process

	BeforeSuite(func() {
		var err error
		metricsServerPath, err = gexec.Build("github.com/cloudfoundry-incubator/runtime-metrics-server")
		Ω(err).ShouldNot(HaveOccurred())
		metricsPort = 5678 + GinkgoParallelNode()
		etcdPort = 5001 + GinkgoParallelNode()
		natsPort = 4228 + GinkgoParallelNode()
	})

	AfterSuite(func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
		nats = natsrunner.NewNATSRunner(natsPort)
		metricsServer = NewMetricServer(metricsServerPath, metricsPort, etcdPort, natsPort)
	})

	JustBeforeEach(func() {
		metricsProcess = ifrit.Envoke(metricsServer)
	})

	AfterEach(func() {
		defer func() {
			metricsProcess.Signal(os.Kill)
			Eventually(metricsProcess.Wait(), time.Second).Should(Receive())
		}()

		metricsProcess.Signal(os.Interrupt)
		Eventually(metricsProcess.Wait(), 5*time.Second).Should(Receive(BeNil()))
	})

	Context("When nats is avaialble", func() {
		BeforeEach(func() {
			nats.Start()
			etcdRunner.Start()
		})

		AfterEach(func() {
			nats.Stop()
			etcdRunner.Stop()
		})

		It("reports started", func() {
			Eventually(metricsServer).Should(gbytes.Say("started"))
		})

		Context("and we are subsribed to component announcements", func() {
			var reg collector_registrar.AnnounceComponentMessage
			var receivedAnnounce chan bool

			BeforeEach(func() {
				receivedAnnounce = make(chan bool)
				nats.MessageBus.Subscribe("vcap.component.announce", func(message *yagnats.Message) {
					err := json.Unmarshal(message.Payload, &reg)
					Ω(err).ShouldNot(HaveOccurred())

					receivedAnnounce <- true
				})
			})

			JustBeforeEach(func() {
				Eventually(receivedAnnounce).Should(Receive())
			})

			It("reports the correct index", func() {
				Ω(reg.Index).Should(Equal(uint(5)))
			})

			It("listens on /varz of the reported host", func() {
				Eventually(func() error {
					conn, err := net.Dial("tcp", reg.Host)
					if err != nil {
						return nil
					}
					defer conn.Close()
					return err
				}).ShouldNot(HaveOccurred())

				req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/varz", reg.Host), nil)
				Ω(err).ShouldNot(HaveOccurred())
				req.SetBasicAuth("the-username", "the-password")

				resp, err := http.DefaultClient.Do(req)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(resp.StatusCode).Should(Equal(200))
			})
		})
	})

	Context("When nats is not avaiable", func() {
		BeforeEach(func() {
			etcdRunner.Start()
		})

		AfterEach(func() {
			etcdRunner.Stop()
		})

		It("should not start server", func() {
			Consistently(metricsServer).ShouldNot(gbytes.Say("started"))
		})

		It("does not exit", func() {
			Consistently(metricsProcess.Wait()).ShouldNot(Receive())
		})
	})

	Context("When nats not available at first, but eventually becomes available", func() {
		BeforeEach(func() {
			etcdRunner.Start()
		})

		AfterEach(func() {
			nats.Stop()
			etcdRunner.Stop()
		})

		It("should not start server until nats becomes avaialble", func() {
			Consistently(metricsServer).ShouldNot(gbytes.Say("started"))
			nats.Start()
			Eventually(metricsServer).Should(gbytes.Say("started"))
		})
	})

})
