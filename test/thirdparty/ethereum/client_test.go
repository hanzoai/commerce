package test

import (
	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("client.GasPrice2", func() {
	It("should work", func() {
		gasPrice, res, err := client.GasPrice2()
		Expect(gasPrice).ToNot(Equal(0))
		Expect(res.FastestWait <= res.FastWait).To(BeTrue())
		Expect(res.FastWait <= res.AverageWait).To(BeTrue())
		Expect(res.AverageWait <= res.SafeLowWait).To(BeTrue())
		Expect(res.Fastest >= res.Fast).To(BeTrue())
		Expect(res.Fast >= res.Average).To(BeTrue())
		Expect(res.Average >= res.SafeLow).To(BeTrue())
		Expect(err).ToNot(HaveOccurred())
	})
})
