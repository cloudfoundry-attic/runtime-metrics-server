package metrics_test

import (
	"errors"
	"os"
	"time"

	. "github.com/cloudfoundry-incubator/runtime-metrics-server/metrics"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs/fake_bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gunk/timeprovider/faketimeprovider"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// a bit of grace time for eventuallys
const aBit = 50 * time.Millisecond

var _ = Describe("PeriodicMetronNotifier", func() {
	var (
		sender *fake.FakeMetricSender

		bbs              *fake_bbs.FakeMetricsBBS
		reportInterval   time.Duration
		fakeTimeProvider *faketimeprovider.FakeTimeProvider

		pmn ifrit.Process
	)

	BeforeEach(func() {
		reportInterval = 100 * time.Millisecond

		bbs = new(fake_bbs.FakeMetricsBBS)
		fakeTimeProvider = faketimeprovider.New(time.Unix(123, 456))

		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender)
	})

	JustBeforeEach(func() {
		pmn = ifrit.Invoke(PeriodicMetronNotifier{
			Interval:     reportInterval,
			MetricsBBS:   bbs,
			TimeProvider: fakeTimeProvider,
		})
	})

	AfterEach(func() {
		pmn.Signal(os.Interrupt)
		Eventually(pmn.Wait()).Should(Receive())
	})

	Context("when the report interval elapses", func() {
		JustBeforeEach(func() {
			fakeTimeProvider.Increment(reportInterval)
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
					fakeTimeProvider.Increment(time.Hour)

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
