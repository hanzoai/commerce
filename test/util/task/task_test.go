package test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/util/log"
	"crowdstart.io/util/task"
)

func TestDatastore(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	RegisterFailHandler(Fail)
	RunSpecs(t, "util/task")
}

// var (
// 	ctx aetest.Context
// )

// var _ = BeforeSuite(func() {
// 	var err error
// 	ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})
// 	Expect(err).ToNot(HaveOccurred())
// })

// var _ = AfterSuite(func() {
// 	err := ctx.Close()
// 	Expect(err).ToNot(HaveOccurred())
// })

var _ = Describe("Register", func() {
	It("Should add task to registry", func() {
		Expect(len(task.Registry["foo"])).To(Equal(0))

	})

	It("Should append additional tasks when name is re-used", func() {
		task.Register("foo", struct{ x string }{"foo"})
		Expect(len(task.Registry["foo"])).To(Equal(1))

		task.Register("foo", struct{ x string }{"foo"})
		Expect(len(task.Registry["foo"])).To(Equal(2))

		for _, v := range task.Registry["foo"] {
			foo, _ := v.(struct{ x string })
			Expect(foo).To(Equal(struct{ x string }{"foo"}))
		}
	})
})
