package test

import (
	"math/big"

	"hanzo.io/models/blockchains"
	"hanzo.io/models/order"
	"hanzo.io/models/payment"
	"hanzo.io/models/types/currency"

	. "hanzo.io/thirdparty/ethereum/tasks"
	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("thirdparty/ethereum/tasks/pay.go", func() {
	Context("tasks.EthereumProcessPayment", func() {
		It("Should Create a Payment", func() {
			txHash := "testHash123"
			chainType := blockchains.EthereumRopstenType
			err := EthereumProcessPaymentImpl(
				ctx,
				"suchtees",
				w.Id(),
				"testHash123",
				string(chainType),
				big.NewInt(123e3*1e9),
			)

			Expect(err).ToNot(HaveOccurred())

			ord2 := order.New(nsDb)
			ok, err := ord2.Query().Filter("WalletId=", w.Id()).Get()
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			Expect(ord2.Paid).To(Equal(currency.Cents(123e3)))
			Expect(ord2.PaymentStatus).To(Equal(payment.Paid))
			Expect(len(ord2.PaymentIds)).To(Equal(1))

			pay2 := payment.New(nsDb)
			ok, err = pay2.Query().Filter("Account.EthereumTransactionHash=", txHash).Get()
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(ord2.PaymentIds[0]).To(Equal(pay2.Id()))

			Expect(pay2.Account.EthereumTransactionHash).To(Equal(txHash))
			Expect(pay2.Account.EthereumChainType).To(Equal(chainType))
			Expect(pay2.Account.EthereumAmount).To(Equal(blockchains.BigNumber(big.NewInt(123e3 * 1e9).String())))

			Expect(pay2.Account.EthereumFinalTransactionHash).To(Equal("0x0"))
			Expect(pay2.Account.EthereumFinalAddress).To(Equal("0xf2fccc0198fc6b39246bd91272769d46d2f9d43b"))
			Expect(pay2.Account.EthereumFinalAmount).To(Equal(blockchains.BigNumber(big.NewInt(.95*123e3*1e9 - 21000).String())))
			Expect(pay2.Account.EthereumFinalGasUsed).To(Equal(blockchains.BigNumber(big.NewInt(21000).String())))

			Expect(pay2.Test).To(BeTrue())
			Expect(pay2.Status).To(Equal(payment.Paid))
			Expect(pay2.Type).To(Equal(ord.Type))
			Expect(pay2.Buyer).To(Equal(usr.Buyer()))
			Expect(pay2.Currency).To(Equal(ord.Currency))
			Expect(pay2.OrderId).To(Equal(ord.Id()))
			Expect(pay2.UserId).To(Equal(usr.Id()))
			Expect(pay2.Amount).To(Equal(currency.Cents(123e3)))

			Expect(pay2.Fee).To(Equal(currency.Cents(.05 * 123e3)))
			Expect(len(pay2.FeeIds)).To(Equal(1))

			fees, err := pay2.GetFees()
			Expect(err).ToNot(HaveOccurred())

			fe := fees[0]
			Expect(fe.Currency).To(Equal(currency.ETH))
			Expect(fe.Amount).To(Equal(currency.Cents(.05 * 123e3)))

			Expect(fe.Ethereum.FinalTransactionHash).To(Equal("0x0"))
			Expect(fe.Ethereum.FinalAddress).To(Equal(pw.Accounts[0].Address))
			Expect(fe.Ethereum.FinalAmount).To(Equal(blockchains.BigNumber(big.NewInt(.05*123e3*1e9 - 21000).String())))
			Expect(fe.Ethereum.FinalGasUsed).To(Equal(blockchains.BigNumber(big.NewInt(21000).String())))
		})

		It("Should Create a Multiple Payments", func() {
			txHash1 := "testHash123"
			txHash2 := "testHash1234"
			chainType := blockchains.EthereumRopstenType
			err := EthereumProcessPaymentImpl(
				ctx,
				"suchtees",
				w.Id(),
				"testHash123",
				string(chainType),
				big.NewInt(123e3*1e9),
			)

			Expect(err).ToNot(HaveOccurred())

			err = EthereumProcessPaymentImpl(
				ctx,
				"suchtees",
				w.Id(),
				"testHash1234",
				string(chainType),
				big.NewInt(321e3*1e9),
			)

			Expect(err).ToNot(HaveOccurred())

			ord2 := order.New(nsDb)
			ok, err := ord2.Query().Filter("WalletId=", w.Id()).Get()
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			Expect(ord2.Paid).To(Equal(currency.Cents(444e3)))
			Expect(ord2.PaymentStatus).To(Equal(payment.Paid))
			Expect(len(ord2.PaymentIds)).To(Equal(2))

			pay1 := payment.New(nsDb)
			ok, err = pay1.Query().Filter("Account.EthereumTransactionHash=", txHash1).Get()
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(ord2.PaymentIds[0]).To(Equal(pay1.Id()))

			Expect(pay1.Account.EthereumTransactionHash).To(Equal(txHash1))
			Expect(pay1.Account.EthereumChainType).To(Equal(chainType))
			Expect(pay1.Account.EthereumAmount).To(Equal(blockchains.BigNumber(big.NewInt(123e3 * 1e9).String())))

			Expect(pay1.Account.EthereumFinalTransactionHash).To(Equal("0x0"))
			Expect(pay1.Account.EthereumFinalAddress).To(Equal("0xf2fccc0198fc6b39246bd91272769d46d2f9d43b"))
			Expect(pay1.Account.EthereumFinalAmount).To(Equal(blockchains.BigNumber(big.NewInt(.95*123e3*1e9 - 21000).String())))
			Expect(pay1.Account.EthereumFinalGasUsed).To(Equal(blockchains.BigNumber(big.NewInt(21000).String())))

			Expect(pay1.Fee).To(Equal(currency.Cents(.05 * 123e3)))
			Expect(len(pay1.FeeIds)).To(Equal(1))

			fees1, err := pay1.GetFees()
			Expect(err).ToNot(HaveOccurred())

			fe1 := fees1[0]
			Expect(fe1.Currency).To(Equal(currency.ETH))
			Expect(fe1.Amount).To(Equal(currency.Cents(.05 * 123e3)))

			Expect(fe1.Ethereum.FinalTransactionHash).To(Equal("0x0"))
			Expect(fe1.Ethereum.FinalAddress).To(Equal(pw.Accounts[0].Address))
			Expect(fe1.Ethereum.FinalAmount).To(Equal(blockchains.BigNumber(big.NewInt(.05*123e3*1e9 - 21000).String())))
			Expect(fe1.Ethereum.FinalGasUsed).To(Equal(blockchains.BigNumber(big.NewInt(21000).String())))

			Expect(pay1.Test).To(BeTrue())
			Expect(pay1.Status).To(Equal(payment.Paid))
			Expect(pay1.Type).To(Equal(ord.Type))
			Expect(pay1.Buyer).To(Equal(usr.Buyer()))
			Expect(pay1.Currency).To(Equal(ord.Currency))
			Expect(pay1.OrderId).To(Equal(ord.Id()))
			Expect(pay1.UserId).To(Equal(usr.Id()))
			Expect(pay1.Amount).To(Equal(currency.Cents(123e3)))

			pay2 := payment.New(nsDb)
			ok, err = pay2.Query().Filter("Account.EthereumTransactionHash=", txHash2).Get()
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(ord2.PaymentIds[1]).To(Equal(pay2.Id()))

			Expect(pay2.Account.EthereumTransactionHash).To(Equal(txHash2))
			Expect(pay2.Account.EthereumChainType).To(Equal(chainType))
			Expect(pay2.Account.EthereumAmount).To(Equal(blockchains.BigNumber(big.NewInt(321e3 * 1e9).String())))

			Expect(pay2.Account.EthereumFinalTransactionHash).To(Equal(""))
			Expect(pay2.Account.EthereumFinalAddress).To(Equal(""))
			Expect(pay2.Account.EthereumFinalAmount).To(Equal(blockchains.BigNumber("")))
			Expect(pay2.Account.EthereumFinalGasUsed).To(Equal(blockchains.BigNumber("")))

			Expect(pay2.Test).To(BeTrue())
			Expect(pay2.Status).To(Equal(payment.Paid))
			Expect(pay2.Type).To(Equal(ord.Type))
			Expect(pay2.Buyer).To(Equal(usr.Buyer()))
			Expect(pay2.Currency).To(Equal(ord.Currency))
			Expect(pay2.OrderId).To(Equal(ord.Id()))
			Expect(pay2.UserId).To(Equal(usr.Id()))
			Expect(pay2.Amount).To(Equal(currency.Cents(321e3)))

			Expect(pay2.Fee).To(Equal(currency.Cents(0)))
			Expect(len(pay2.FeeIds)).To(Equal(0))
		})
	})
})
