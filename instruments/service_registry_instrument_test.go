package instruments_test

import (
	"errors"
	"github.com/cloudfoundry-incubator/metricz/instrumentation"
	. "github.com/cloudfoundry-incubator/runtime-metrics-server/instruments"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs/fake_bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceRegistryInstrument", func() {
	var instrument instrumentation.Instrumentable
	var fakeBBS *fake_bbs.FakeMetricsBBS

	BeforeEach(func() {
		fakeBBS = fake_bbs.NewFakeMetricsBBS()
		instrument = NewServiceRegistryInstrument(fakeBBS)
	})

	Describe("Emit", func() {

		Context("when the are executors", func() {
			var expectedMetric = instrumentation.Metric{
				Name:  "Executor",
				Value: 2,
			}

			BeforeEach(func() {
				fakeBBS.GetServiceRegistrationsReturns.Registrations = models.ServiceRegistrations{
					{Name: models.ExecutorService, Id: "guid-0"},
					{Name: models.ExecutorService, Id: "guid-1"},
				}
			})
			It("should emit the number of executors", func() {
				context := instrument.Emit()
				Ω(context.Name).Should(Equal("ServiceRegistrations"))
				Ω(context.Metrics).Should(ContainElement(expectedMetric))
			})
		})

		Context("when there are no executors", func() {
			var expectedMetric = instrumentation.Metric{
				Name:  "Executor",
				Value: 0,
			}

			BeforeEach(func() {
				fakeBBS.GetServiceRegistrationsReturns.Registrations = models.ServiceRegistrations{}
			})

			It("should emit 0", func() {
				context := instrument.Emit()
				Ω(context.Name).Should(Equal("ServiceRegistrations"))
				Ω(context.Metrics).Should(ContainElement(expectedMetric))
			})
		})

		Context("when etcd returns an error ", func() {
			var expectedMetric = instrumentation.Metric{
				Name:  "Executor",
				Value: -1,
			}

			BeforeEach(func() {
				fakeBBS.GetServiceRegistrationsReturns.Err = errors.New("pur[l;e")
			})

			It("should emit -1", func() {
				context := instrument.Emit()
				Ω(context.Name).Should(Equal("ServiceRegistrations"))
				Ω(context.Metrics).Should(ContainElement(expectedMetric))
			})
		})

	})
})
