package metrics_test

import (
	"errors"
	"net/http"
	"os"
	"time"

	. "github.com/cloudfoundry-incubator/runtime-metrics-server/metrics"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs/fake_bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

// a bit of grace time for eventuallys
const aBit = 50 * time.Millisecond

var _ = Describe("PeriodicMetronNotifier", func() {
	var (
		sender *fake.FakeMetricSender

		bbs            *fake_bbs.FakeMetricsBBS
		etcdCluster    []string
		reportInterval time.Duration
		fakeClock      *fakeclock.FakeClock

		pmn ifrit.Process
	)

	BeforeEach(func() {
		reportInterval = 100 * time.Millisecond

		bbs = new(fake_bbs.FakeMetricsBBS)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))

		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender)
	})

	JustBeforeEach(func() {
		pmn = ifrit.Invoke(PeriodicMetronNotifier{
			Interval:    reportInterval,
			MetricsBBS:  bbs,
			ETCDCluster: etcdCluster,
			Clock:       fakeClock,
			Logger:      lagertest.NewTestLogger("test"),
		})
	})

	AfterEach(func() {
		pmn.Signal(os.Interrupt)
		Eventually(pmn.Wait()).Should(Receive())
	})

	Context("when the report interval elapses", func() {
		JustBeforeEach(func() {
			fakeClock.Increment(reportInterval)
		})

		Context("when the etcd cluster is around", func() {
			var (
				etcd1 *ghttp.Server
				etcd2 *ghttp.Server
				etcd3 *ghttp.Server
			)

			BeforeEach(func() {
				etcd1 = ghttp.NewServer()
				etcd2 = ghttp.NewServer()
				etcd3 = ghttp.NewServer()

				etcdCluster = []string{
					etcd1.URL(),
					etcd2.URL(),
					etcd3.URL(),
				}
			})

			AfterEach(func() {
				etcd1.Close()
				etcd2.Close()
				etcd3.Close()
			})

			Context("when the etcd server gives valid JSON", func() {
				BeforeEach(func() {
					etcd1.RouteToHandler("GET", "/v2/stats/self", ghttp.RespondWith(200, `
            {
              "name": "node1",
							"id": "node1-id",
              "state": "StateFollower",

              "leaderInfo": {
                "leader": "node2-id",
								"uptime": "17h41m45.103057785s",
							  "startTime": "2015-02-13T01:28:26.657389108Z"
              },

              "recvAppendRequestCnt": 1234,
              "recvPkgRate": 2.0,
              "recvBandwidthRate": 1.2,

              "sendAppendRequestCnt": 4321
            }
	        `))

					etcd1.RouteToHandler("GET", "/v2/stats/leader", func(w http.ResponseWriter, r *http.Request) {
						http.Redirect(w, r, etcd2.URL(), 302)
					})

					etcd2.RouteToHandler("GET", "/v2/stats/self", ghttp.RespondWith(200, `
            {
              "name": "node2",
							"id": "node2-id",
              "state": "StateLeader",

              "leaderInfo": {
                "leader": "node2-id",
								"uptime": "17h41m45.103057785s",
							  "startTime": "2015-02-13T01:28:26.657389108Z"
              },

              "recvAppendRequestCnt": 1234,

              "sendAppendRequestCnt": 4321,
              "sendPkgRate": 5.0,
              "sendBandwidthRate": 3.0
            }
	        `))

					etcd2.RouteToHandler("GET", "/v2/stats/leader", ghttp.RespondWith(200, `
						{
						  "leader": "node2-id",
						  "followers": {
						    "node1-id": {
						      "latency": {
						        "current": 0.153507,
						        "average": 0.14636559394884047,
						        "standardDeviation": 0.15477392607571758,
						        "minimum": 8.4e-05,
						        "maximum": 6.78157
						      },
						      "counts": {
						        "fail": 4,
						        "success": 215000
						      }
						    },
						    "node3-id": {
						      "latency": {
						        "current": 0.052932,
						        "average": 0.13533593782359846,
						        "standardDeviation": 0.18151611603344037,
						        "minimum": 7.3e-05,
						        "maximum": 16.432439
						      },
						      "counts": {
						        "fail": 4,
						        "success": 214969
						      }
						    }
						  }
						}
	        `))

					etcd2.RouteToHandler("GET", "/v2/stats/store", ghttp.RespondWith(200, `
						{
							"getsSuccess": 10195,
							"getsFail": 26705,
							"setsSuccess": 2540,
							"setsFail": 0,
							"deleteSuccess": 0,
							"deleteFail": 0,
							"updateSuccess": 0,
							"updateFail": 0,
							"createSuccess": 18,
							"createFail": 15252,
							"compareAndSwapSuccess": 50350,
							"compareAndSwapFail": 22,
							"compareAndDeleteSuccess": 4,
							"compareAndDeleteFail": 0,
							"expireCount": 1,
							"watchers": 12
						}
					`))

					etcd2.RouteToHandler("GET", "/v2/keys", func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("X-Raft-Term", "123")
					})

					etcd3.RouteToHandler("GET", "/v2/stats/self", ghttp.RespondWith(200, `
            {
              "name": "node3",
							"id": "node3-id",
              "state": "StateFollower",

              "leaderInfo": {
                "leader": "node2-id",
								"uptime": "17h41m45.103057785s",
							  "startTime": "2015-02-13T01:28:26.657389108Z"
              },

              "recvAppendRequestCnt": 1234,
              "recvPkgRate": 2.0,
              "recvBandwidthRate": 0.8,

              "sendAppendRequestCnt": 4321
            }
	        `))

					etcd3.RouteToHandler("GET", "/v2/stats/leader", func(w http.ResponseWriter, r *http.Request) {
						http.Redirect(w, r, etcd2.URL(), 302)
					})
				})

				It("should emit them", func() {
					Eventually(func() fake.Metric {
						return sender.GetValue("ETCDLeader")
					}).Should(Equal(fake.Metric{
						Value: 1,
						Unit:  "Metric",
					}))

					Eventually(func() fake.Metric {
						return sender.GetValue("ETCDReceivedBandwidthRate")
					}).Should(Equal(fake.Metric{
						Value: 2,
						Unit:  "B/s",
					}))

					Eventually(func() fake.Metric {
						return sender.GetValue("ETCDSentBandwidthRate")
					}).Should(Equal(fake.Metric{
						Value: 3,
						Unit:  "B/s",
					}))

					Eventually(func() fake.Metric {
						return sender.GetValue("ETCDReceivedRequestRate")
					}).Should(Equal(fake.Metric{
						Value: 4,
						Unit:  "Req/s",
					}))

					Eventually(func() fake.Metric {
						return sender.GetValue("ETCDSentRequestRate")
					}).Should(Equal(fake.Metric{
						Value: 5,
						Unit:  "Req/s",
					}))

					Eventually(func() fake.Metric {
						return sender.GetValue("ETCDRaftTerm")
					}).Should(Equal(fake.Metric{
						Value: 123,
						Unit:  "Metric",
					}))

					Eventually(func() fake.Metric {
						return sender.GetValue("ETCDWatchers")
					}).Should(Equal(fake.Metric{
						Value: 12,
						Unit:  "Metric",
					}))
				})
			})
		})

		Context("when the read from the store succeeds", func() {
			BeforeEach(func() {
				bbs.TasksReturns([]models.Task{
					models.Task{State: models.TaskStatePending},
					models.Task{State: models.TaskStatePending},
					models.Task{State: models.TaskStatePending},

					models.Task{State: models.TaskStateRunning},

					models.Task{State: models.TaskStateCompleted},
					models.Task{State: models.TaskStateCompleted},
					models.Task{State: models.TaskStateCompleted},
					models.Task{State: models.TaskStateCompleted},

					models.Task{State: models.TaskStateResolving},
					models.Task{State: models.TaskStateResolving},
				}, nil)

				bbs.ServiceRegistrationsReturns(models.ServiceRegistrations{
					{Name: models.CellServiceName, Id: "purple-elephants"},
				}, nil)

				bbs.DomainsReturns([]string{"some-domain", "some-other-domain"}, nil)

				bbs.DesiredLRPsReturns([]models.DesiredLRP{
					{ProcessGuid: "desired-1", Instances: 2},
					{ProcessGuid: "desired-2", Instances: 3},
				}, nil)

				bbs.ActualLRPsStub = func() ([]models.ActualLRP, error) {
					fakeClock.Increment(time.Hour)

					return []models.ActualLRP{
						{ActualLRPKey: models.NewActualLRPKey("desired-1", 0, "domain"), State: models.ActualLRPStateRunning},
						{ActualLRPKey: models.NewActualLRPKey("desired-1", 1, "domain"), State: models.ActualLRPStateRunning},
						{ActualLRPKey: models.NewActualLRPKey("desired-2", 1, "domain"), State: models.ActualLRPStateClaimed},
						{ActualLRPKey: models.NewActualLRPKey("desired-3", 0, "domain"), State: models.ActualLRPStateRunning},
						{ActualLRPKey: models.NewActualLRPKey("desired-3", 1, "domain"), State: models.ActualLRPStateCrashed},
						{ActualLRPKey: models.NewActualLRPKey("desired-3", 2, "domain"), State: models.ActualLRPStateCrashed},
						{ActualLRPKey: models.NewActualLRPKey("desired-4", 0, "domain"), State: models.ActualLRPStateCrashed},
					}, nil
				}
			})

			It("reports how long it took to emit", func() {
				Eventually(func() fake.Metric {
					return sender.GetValue("MetricsReportingDuration")
				}).Should(Equal(fake.Metric{
					Value: float64(1 * time.Hour),
					Unit:  "nanos",
				}))
			})

			It("reports the number of registered services by type", func() {
				Eventually(func() fake.Metric {
					return sender.GetValue("ServiceRegistrationsCell")
				}).Should(Equal(fake.Metric{
					Value: 1,
					Unit:  "Metric",
				}))
			})

			It("reports that the store's domains are fresh", func() {
				Eventually(func() fake.Metric {
					return sender.GetValue("Domain.some-domain")
				}).Should(Equal(fake.Metric{
					Value: 1,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("Domain.some-other-domain")
				}).Should(Equal(fake.Metric{
					Value: 1,
					Unit:  "Metric",
				}))
			})

			It("emits metrics for tasks in each state", func() {
				Eventually(func() fake.Metric {
					return sender.GetValue("TasksPending")
				}).Should(Equal(fake.Metric{
					Value: 3,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("TasksRunning")
				}).Should(Equal(fake.Metric{
					Value: 1,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("TasksCompleted")
				}).Should(Equal(fake.Metric{
					Value: 4,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("TasksResolving")
				}).Should(Equal(fake.Metric{
					Value: 2,
					Unit:  "Metric",
				}))
			})

			It("emits desired LRP metrics", func() {
				Eventually(func() fake.Metric {
					return sender.GetValue("LRPsDesired")
				}).Should(Equal(fake.Metric{
					Value: 5,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("LRPsStarting")
				}).Should(Equal(fake.Metric{
					Value: 1,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("LRPsRunning")
				}).Should(Equal(fake.Metric{
					Value: 3,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("CrashedActualLRPs")
				}).Should(Equal(fake.Metric{
					Value: 3,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("CrashingDesiredLRPs")
				}).Should(Equal(fake.Metric{
					Value: 2,
					Unit:  "Metric",
				}))
			})
		})

		Context("when the store cannot be reached", func() {
			BeforeEach(func() {
				bbs.TasksReturns(nil, errors.New("Doesn't work"))
				bbs.DesiredLRPsReturns(nil, errors.New("no."))
				bbs.ActualLRPsReturns(nil, errors.New("pushed to master"))
			})

			It("reports -1 for all task metrics", func() {
				Eventually(func() fake.Metric {
					return sender.GetValue("TasksPending")
				}).Should(Equal(fake.Metric{
					Value: -1,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("TasksRunning")
				}).Should(Equal(fake.Metric{
					Value: -1,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("TasksCompleted")
				}).Should(Equal(fake.Metric{
					Value: -1,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("TasksResolving")
				}).Should(Equal(fake.Metric{
					Value: -1,
					Unit:  "Metric",
				}))
			})

			It("reports -1 for all LRP metrics", func() {
				Eventually(func() fake.Metric {
					return sender.GetValue("LRPsDesired")
				}).Should(Equal(fake.Metric{
					Value: -1,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("LRPsStarting")
				}).Should(Equal(fake.Metric{
					Value: -1,
					Unit:  "Metric",
				}))

				Eventually(func() fake.Metric {
					return sender.GetValue("LRPsRunning")
				}).Should(Equal(fake.Metric{
					Value: -1,
					Unit:  "Metric",
				}))
			})
		})
	})
})
