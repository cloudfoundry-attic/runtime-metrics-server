package metrics_test

import (
	"errors"
	"os"
	"time"

	. "github.com/cloudfoundry-incubator/runtime-metrics-server/metrics"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs/fake_bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry/dropsonde/autowire/metrics"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// a bit of grace time for eventuallys
const aBit = 50 * time.Millisecond

var _ = Describe("PeriodicMetronNotifier", func() {
	var (
		sender *fake.FakeMetricSender

		bbs            *fake_bbs.FakeMetricsBBS
		reportInterval time.Duration

		pmn ifrit.Process
	)

	BeforeEach(func() {
		reportInterval = 100 * time.Millisecond

		bbs = new(fake_bbs.FakeMetricsBBS)

		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender)
	})

	JustBeforeEach(func() {
		pmn = ifrit.Envoke(PeriodicMetronNotifier{
			Interval:   reportInterval,
			MetricsBBS: bbs,
		})
	})

	AfterEach(func() {
		pmn.Signal(os.Interrupt)
		Eventually(pmn.Wait()).Should(Receive())
	})

	Context("when the read from the store succeeds", func() {
		BeforeEach(func() {
			bbs.GetAllTasksReturns([]models.Task{
				models.Task{State: models.TaskStatePending},
				models.Task{State: models.TaskStatePending},
				models.Task{State: models.TaskStatePending},

				models.Task{State: models.TaskStateClaimed},
				models.Task{State: models.TaskStateClaimed},

				models.Task{State: models.TaskStateRunning},

				models.Task{State: models.TaskStateCompleted},
				models.Task{State: models.TaskStateCompleted},
				models.Task{State: models.TaskStateCompleted},
				models.Task{State: models.TaskStateCompleted},

				models.Task{State: models.TaskStateResolving},
				models.Task{State: models.TaskStateResolving},
			}, nil)

			bbs.GetServiceRegistrationsReturns(models.ServiceRegistrations{
				{Name: models.ExecutorServiceName, Id: "purple-elephants"},
				{Name: models.FileServerServiceName, Id: "plurple-elephants"},
				{Name: models.FileServerServiceName, Id: "red-elephants"},
			}, nil)

			bbs.GetAllFreshnessReturns([]string{"some-domain", "some-other-domain"}, nil)

			bbs.GetAllDesiredLRPsReturns([]models.DesiredLRP{
				{ProcessGuid: "desired-1", Instances: 2},
				{ProcessGuid: "desired-2", Instances: 3},
			}, nil)

			bbs.GetAllActualLRPsReturns([]models.ActualLRP{
				{ProcessGuid: "desired-1", Index: 0, State: models.ActualLRPStateRunning},
				{ProcessGuid: "desired-1", Index: 1, State: models.ActualLRPStateRunning},
				{ProcessGuid: "desired-2", Index: 1, State: models.ActualLRPStateStarting},
			}, nil)
		})

		It("reports the number of registered services by type", func() {
			Eventually(func() fake.Metric {
				return sender.GetValue("service-registrations-Executor")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: 1,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("service-registrations-FileServer")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: 2,
				Unit:  "Metric",
			}))
		})

		It("reports that the store's domains are fresh", func() {
			Eventually(func() fake.Metric {
				return sender.GetValue("freshness-some-domain")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: 1,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("freshness-some-other-domain")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: 1,
				Unit:  "Metric",
			}))
		})

		It("emits metrics for tasks in each state", func() {
			Eventually(func() fake.Metric {
				return sender.GetValue("pending-tasks")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: 3,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("claimed-tasks")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: 2,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("running-tasks")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: 1,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("completed-tasks")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: 4,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("resolving-tasks")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: 2,
				Unit:  "Metric",
			}))
		})

		It("emits desired LRP metrics", func() {
			Eventually(func() fake.Metric {
				return sender.GetValue("desired-lrps")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: 5,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("starting-lrps")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: 1,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("running-lrps")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: 2,
				Unit:  "Metric",
			}))
		})
	})

	Context("when the store cannot be reached", func() {
		BeforeEach(func() {
			bbs.GetAllTasksReturns(nil, errors.New("Doesn't work"))
			bbs.GetAllDesiredLRPsReturns(nil, errors.New("no."))
			bbs.GetAllActualLRPsReturns(nil, errors.New("pushed to master"))
		})

		It("reports -1 for all task metrics", func() {
			Eventually(func() fake.Metric {
				return sender.GetValue("pending-tasks")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: -1,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("claimed-tasks")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: -1,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("running-tasks")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: -1,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("completed-tasks")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: -1,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("resolving-tasks")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: -1,
				Unit:  "Metric",
			}))
		})

		It("reports -1 for all LRP metrics", func() {
			Eventually(func() fake.Metric {
				return sender.GetValue("desired-lrps")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: -1,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("starting-lrps")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: -1,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("running-lrps")
			}, reportInterval+aBit).Should(Equal(fake.Metric{
				Value: -1,
				Unit:  "Metric",
			}))
		})
	})
})
