package main_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"

	"github.com/cloudfoundry-incubator/metricz/collector_registrar"
	"github.com/cloudfoundry/gunk/natsrunner"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/cloudfoundry/yagnats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestRuntimeMetricsServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RuntimeMetricsServer Suite")
}

var _ = Describe("Main", func() {
	var nats *natsrunner.NATSRunner
	var etcdRunner *etcdstorerunner.ETCDClusterRunner

	BeforeEach(func() {
		nats = natsrunner.NewNATSRunner(4228)
		nats.Start()
		etcdRunner = etcdstorerunner.NewETCDClusterRunner(5001, 1)
		etcdRunner.Start()
	})

	AfterEach(func() {
		nats.Stop()
		etcdRunner.Stop()
	})

	It("starts the server correctly", func() {
		var reg collector_registrar.AnnounceComponentMessage

		receivedAnnounce := make(chan bool)

		nats.MessageBus.Subscribe("vcap.component.announce", func(message *yagnats.Message) {
			err := json.Unmarshal(message.Payload, &reg)
			Ω(err).ShouldNot(HaveOccurred())

			receivedAnnounce <- true
		})

		metricsServerPath, err := gexec.Build("github.com/cloudfoundry-incubator/runtime-metrics-server")
		Ω(err).ShouldNot(HaveOccurred())

		serverCmd := exec.Command(
			metricsServerPath,
			"-port", "5678",
			"-etcdCluster", "http://127.0.0.1:5001",
			"-natsAddresses", "127.0.0.1:4228",
			"-index", "5",
			"-username", "the-username",
			"-password", "the-password",
		)
		serverCmd.Env = os.Environ()

		session, err := gexec.Start(serverCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())
		defer func() {
			session.Kill().Wait()
		}()

		Eventually(receivedAnnounce, 5).Should(Receive())

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

		Ω(reg.Index).Should(Equal(uint(5)))

		resp, err := http.DefaultClient.Do(req)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(resp.StatusCode).Should(Equal(200))
	})
})
