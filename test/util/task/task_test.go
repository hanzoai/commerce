package test

import (
	"testing"

	"crowdstart.com/util/task"
	. "crowdstart.com/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/task", t)
}

type X struct {
	x string
}

var _ = Describe("Register", func() {
	It("Should add task to registry", func() {
		Expect(len(task.Registry["foo"])).To(Equal(0))
		task.Register("foo", X{"foo"})
		Expect(len(task.Registry["foo"])).To(Equal(1))
	})

	It("Should append additional tasks when name is re-used", func() {
		task.Register("bar", X{"bar"})
		Expect(len(task.Registry["bar"])).To(Equal(1))

		task.Register("bar", X{"bar"})
		Expect(len(task.Registry["bar"])).To(Equal(2))

		for _, v := range task.Registry["bar"] {
			bar, _ := v.(X)
			Expect(bar).To(Equal(X{"bar"}))
		}
	})

	It("Should append multiple tasks at once", func() {
		task.Register("baz", X{"baz"}, X{"baz"}, X{"baz"})
		Expect(len(task.Registry["baz"])).To(Equal(3))

		for _, v := range task.Registry["baz"] {
			baz, _ := v.(X)
			Expect(baz).To(Equal(X{"baz"}))
		}
	})
})
