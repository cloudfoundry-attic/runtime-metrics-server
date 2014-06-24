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
		var context instrumentation.Context

		JustBeforeEach(func() {
			context = instrument.Emit()
		})

		Context("when the are services", func() {
			BeforeEach(func() {
				fakeBBS.GetServiceRegistrationsReturns.Registrations = models.ServiceRegistrations{
					{Name: models.ExecutorServiceName, Id: "guid-0"},
					{Name: models.ExecutorServiceName, Id: "guid-1"},
					{Name: models.FileServerServiceName, Id: "guid-0"},
				}
			})

			It("should have a name", func() {
				Ω(context.Name).Should(Equal("ServiceRegistrations"))
			})

			It("should emit the number of executors", func() {
				Ω(context.Name).Should(Equal("ServiceRegistrations"))
				Ω(context.Metrics).Should(ContainElement(instrumentation.Metric{Name: "Executor", Value: 2}))
			})

			It("should emit the number of file servers", func() {
				Ω(context.Name).Should(Equal("ServiceRegistrations"))
				Ω(context.Metrics).Should(ContainElement(instrumentation.Metric{Name: "FileServer", Value: 1}))
			})
		})

		Context("when there are no services", func() {
			BeforeEach(func() {
				fakeBBS.GetServiceRegistrationsReturns.Registrations = models.ServiceRegistrations{}
			})

			It("should emit 0 executors", func() {
				Ω(context.Metrics).Should(ContainElement(instrumentation.Metric{Name: "Executor", Value: 0}))
			})

			It("should emit 0 file servers", func() {
				Ω(context.Metrics).Should(ContainElement(instrumentation.Metric{Name: "FileServer", Value: 0}))
			})
		})

		Context("when etcd returns an error ", func() {
			BeforeEach(func() {
				fakeBBS.GetServiceRegistrationsReturns.Err = errors.New("pur[l;e")
			})

			It("should emit -1 executors", func() {
				Ω(context.Metrics).Should(ContainElement(instrumentation.Metric{Name: "Executor", Value: -1}))
			})

			It("should emit 0 file servers", func() {
				Ω(context.Metrics).Should(ContainElement(instrumentation.Metric{Name: "FileServer", Value: -1}))
			})
		})

	})
})
