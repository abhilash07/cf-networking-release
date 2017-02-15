package store_test

import (
	"errors"
	"policy-server/models"
	"policy-server/store"
	"policy-server/store/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetricsWrapper", func() {
	var (
		metricsWrapper    *store.MetricsWrapper
		policies          []models.Policy
		policiesFilter    models.PoliciesFilter
		tags              []models.Tag
		fakeMetricsSender *fakes.MetricsSender
		fakeStore         *fakes.Store
	)

	BeforeEach(func() {
		fakeStore = &fakes.Store{}
		fakeMetricsSender = &fakes.MetricsSender{}
		metricsWrapper = &store.MetricsWrapper{
			Store:         fakeStore,
			MetricsSender: fakeMetricsSender,
		}
		policies = []models.Policy{{
			Source: models.Source{ID: "some-app-guid"},
			Destination: models.Destination{
				ID:       "some-other-app-guid",
				Protocol: "tcp",
				Port:     8080,
			},
		}}
		policiesFilter = models.PoliciesFilter{
			SourceGuids:      []string{"some-app-guid"},
			DestinationGuids: []string{"some-other-app-guid"},
		}
		tags = []models.Tag{{
			ID:  "some-app-guid",
			Tag: "0001",
		}, {
			ID:  "some-other-app-guid",
			Tag: "0002",
		}}
	})

	Describe("Create", func() {
		It("calls Create on the Store", func() {
			err := metricsWrapper.Create(policies)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeStore.CreateCallCount()).To(Equal(1))
			Expect(fakeStore.CreateArgsForCall(0)).To(Equal(policies))
		})

		It("emits a metric", func() {
			err := metricsWrapper.Create(policies)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
			name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
			Expect(name).To(Equal("StoreCreateTime"))
		})

		Context("when there is an error", func() {
			BeforeEach(func() {
				fakeStore.CreateReturns(errors.New("banana"))
			})
			It("emits an error metric", func() {
				err := metricsWrapper.Create(policies)
				Expect(err).To(MatchError("banana"))

				Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
				Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("StoreCreateError"))

				Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
				name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
				Expect(name).To(Equal("StoreCreateTime"))
			})
		})
	})

	Describe("All", func() {
		BeforeEach(func() {
			fakeStore.AllReturns(policies, nil)
		})
		It("returns the result of All on the Store", func() {
			returnedPolicies, err := metricsWrapper.All()
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedPolicies).To(Equal(policies))

			Expect(fakeStore.AllCallCount()).To(Equal(1))
		})

		It("emits a metric", func() {
			_, err := metricsWrapper.All()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
			name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
			Expect(name).To(Equal("StoreAllTime"))
		})

		Context("when there is an error", func() {
			BeforeEach(func() {
				fakeStore.AllReturns(nil, errors.New("banana"))
			})
			It("emits an error metric", func() {
				_, err := metricsWrapper.All()
				Expect(err).To(MatchError("banana"))

				Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
				Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("StoreAllError"))

				Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
				name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
				Expect(name).To(Equal("StoreAllTime"))

			})
		})
	})

	Describe("PoliciesWithFilter", func() {
		BeforeEach(func() {
			fakeStore.PoliciesWithFilterReturns(policies, nil)
		})
		It("returns the result of PoliciesWithFilter on the Store", func() {
			returnedPolicies, err := metricsWrapper.PoliciesWithFilter(policiesFilter)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedPolicies).To(Equal(policies))

			Expect(fakeStore.PoliciesWithFilterCallCount()).To(Equal(1))
			Expect(fakeStore.PoliciesWithFilterArgsForCall(0)).To(Equal(policiesFilter))
		})

		It("emits a metric", func() {
			_, err := metricsWrapper.PoliciesWithFilter(policiesFilter)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
			name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
			Expect(name).To(Equal("StorePoliciesWithFilterTime"))
		})

		Context("when there is an error", func() {
			BeforeEach(func() {
				fakeStore.PoliciesWithFilterReturns(nil, errors.New("banana"))
			})
			It("emits an error metric", func() {
				_, err := metricsWrapper.PoliciesWithFilter(policiesFilter)
				Expect(err).To(MatchError("banana"))

				Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
				Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("StorePoliciesWithFilterError"))

				Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
				name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
				Expect(name).To(Equal("StorePoliciesWithFilterTime"))

			})
		})
	})

	Describe("Delete", func() {
		It("calls Delete on the Store", func() {
			err := metricsWrapper.Delete(policies)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeStore.DeleteCallCount()).To(Equal(1))
			Expect(fakeStore.DeleteArgsForCall(0)).To(Equal(policies))
		})

		It("emits a metric", func() {
			err := metricsWrapper.Delete(policies)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
			name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
			Expect(name).To(Equal("StoreDeleteTime"))
		})

		Context("when there is an error", func() {
			BeforeEach(func() {
				fakeStore.DeleteReturns(errors.New("banana"))
			})
			It("emits an error metric", func() {
				err := metricsWrapper.Delete(policies)
				Expect(err).To(MatchError("banana"))

				Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
				Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("StoreDeleteError"))

				Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
				name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
				Expect(name).To(Equal("StoreDeleteTime"))
			})
		})
	})

	Describe("Tags", func() {
		BeforeEach(func() {
			fakeStore.TagsReturns(tags, nil)
		})
		It("calls Tags on the Store", func() {
			returnedTags, err := metricsWrapper.Tags()
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedTags).To(Equal(tags))

			Expect(fakeStore.TagsCallCount()).To(Equal(1))
		})

		It("emits a metric", func() {
			_, err := metricsWrapper.Tags()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
			name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
			Expect(name).To(Equal("StoreTagsTime"))

		})

		Context("when there is an error", func() {
			BeforeEach(func() {
				fakeStore.TagsReturns(nil, errors.New("banana"))
			})
			It("emits an error metric", func() {
				_, err := metricsWrapper.Tags()
				Expect(err).To(MatchError("banana"))

				Expect(fakeMetricsSender.IncrementCounterCallCount()).To(Equal(1))
				Expect(fakeMetricsSender.IncrementCounterArgsForCall(0)).To(Equal("StoreTagsError"))

				Expect(fakeMetricsSender.SendDurationCallCount()).To(Equal(1))
				name, _ := fakeMetricsSender.SendDurationArgsForCall(0)
				Expect(name).To(Equal("StoreTagsTime"))

			})
		})
	})
})
