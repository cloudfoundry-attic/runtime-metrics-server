package metrics_server_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudfoundry-incubator/metricz/localip"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs/fake_bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"

	"github.com/cloudfoundry-incubator/metricz/instrumentation"
	. "github.com/cloudfoundry-incubator/runtime-metrics-server/metrics_server"
	steno "github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics Server", func() {
	var (
		fakenats   *fakeyagnats.FakeYagnats
		logger     *steno.Logger
		bbs        *fake_bbs.FakeMetricsBBS
		server     *MetricsServer
		httpClient *http.Client
	)

	BeforeEach(func() {
		fakenats = fakeyagnats.New()
		bbs = fake_bbs.NewFakeMetricsBBS()
		logger = steno.NewLogger("fakelogger")
		server = New(fakenats, bbs, logger, Config{
			Port:     34567,
			Username: "the-username",
			Password: "the-password",
			Index:    3,
		})

		httpClient = &http.Client{
			Transport: &http.Transport{},
		}
	})

	Describe("Listen", func() {
		var (
			payloadChan chan []byte
			myIP        string
		)

		BeforeEach(func() {
			var err error
			myIP, err = localip.LocalIP()
			Ω(err).ShouldNot(HaveOccurred())

			payloadChan = make(chan []byte, 1)
			fakenats.Subscribe("vcap.component.announce", func(msg *yagnats.Message) {
				payloadChan <- msg.Payload
			})

			go func() {
				err := server.Listen()
				Ω(err).ShouldNot(HaveOccurred())
			}()
		})

		AfterEach(func() {
			server.Stop()
		})

		It("announces to the collector with the right type, port, credentials and index", func(done Done) {
			payload := <-payloadChan
			response := make(map[string]interface{})
			json.Unmarshal(payload, &response)

			Ω(response["type"]).Should(Equal("Runtime-Metrics"))

			Ω(strings.HasSuffix(response["host"].(string), ":34567")).Should(BeTrue())

			Ω(response["credentials"]).Should(Equal([]interface{}{
				"the-username",
				"the-password",
			}))

			Ω(response["index"]).Should(Equal(float64(3)))

			close(done)
		}, 3)

		Describe("the varz endpoint", func() {
			getVarz := func() instrumentation.VarzMessage {
				request, _ := http.NewRequest("GET", "http://"+myIP+":34567/varz", nil)
				request.SetBasicAuth("the-username", "the-password")

				response, err := httpClient.Do(request)
				Ω(err).ShouldNot(HaveOccurred())
				bytes, _ := ioutil.ReadAll(response.Body)
				varzMessage := instrumentation.VarzMessage{}

				err = json.Unmarshal(bytes, &varzMessage)
				Ω(err).ShouldNot(HaveOccurred())

				return varzMessage
			}

			Context("when the read from the store succeeds", func() {
				BeforeEach(func() {
					bbs.GetAllRunOncesReturns.Models = []*models.RunOnce{
						&models.RunOnce{State: models.RunOnceStatePending},
						&models.RunOnce{State: models.RunOnceStatePending},
						&models.RunOnce{State: models.RunOnceStatePending},

						&models.RunOnce{State: models.RunOnceStateClaimed},
						&models.RunOnce{State: models.RunOnceStateClaimed},

						&models.RunOnce{State: models.RunOnceStateRunning},

						&models.RunOnce{State: models.RunOnceStateCompleted},
						&models.RunOnce{State: models.RunOnceStateCompleted},
						&models.RunOnce{State: models.RunOnceStateCompleted},
						&models.RunOnce{State: models.RunOnceStateCompleted},

						&models.RunOnce{State: models.RunOnceStateResolving},
						&models.RunOnce{State: models.RunOnceStateResolving},
					}
				})

				It("returns the number of tasks in each state", func() {
					varzMessage := getVarz()
					Ω(varzMessage.Name).Should(Equal("Runtime-Metrics"))
					Ω(varzMessage.Contexts[0]).Should(Equal(instrumentation.Context{
						Name: "Tasks",
						Metrics: []instrumentation.Metric{
							{
								Name:  "Pending",
								Value: float64(3),
							},
							{
								Name:  "Claimed",
								Value: float64(2),
							},
							{
								Name:  "Running",
								Value: float64(1),
							},
							{
								Name:  "Completed",
								Value: float64(4),
							},
							{
								Name:  "Resolving",
								Value: float64(2),
							},
						},
					}))
				})
			})

			Context("when there is an error reading from the store", func() {
				BeforeEach(func() {
					bbs.GetAllRunOncesReturns.Err = errors.New("Doesn't work")
				})

				It("reports -1 for all of the task counts", func() {
					varzMessage := getVarz()
					Ω(varzMessage.Contexts[0]).Should(Equal(instrumentation.Context{
						Name: "Tasks",
						Metrics: []instrumentation.Metric{
							{
								Name:  "Pending",
								Value: float64(-1),
							},
							{
								Name:  "Claimed",
								Value: float64(-1),
							},
							{
								Name:  "Running",
								Value: float64(-1),
							},
							{
								Name:  "Completed",
								Value: float64(-1),
							},
							{
								Name:  "Resolving",
								Value: float64(-1),
							},
						},
					}))
				})
			})
		})

		Describe("the healthz endpoint", func() {
			It("returns success", func() {
				request, _ := http.NewRequest("GET", "http://"+myIP+":34567/healthz", nil)
				request.SetBasicAuth("the-username", "the-password")

				response, err := httpClient.Do(request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response.StatusCode).To(Equal(200))

				body, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(string(body)).Should(Equal("ok"))
			})
		})
	})
})
