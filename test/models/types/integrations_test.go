package test

import (
	// "hanzo.io/util/log"
	"regexp"

	. "hanzo.io/models/types/integrations"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("models/types/integrations", func() {
	Context("Append", func() {
		It("should return integrations with new one appended", func() {
			ins := Integrations{}
			Expect(len(ins)).To(Equal(0))
			ins = ins.MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			})
			Expect(len(ins)).To(Equal(1))

			in := ins[0]

			Expect(in.CreatedAt.IsZero()).To(BeFalse())
			Expect(in.UpdatedAt.IsZero()).To(BeFalse())
		})

		It("should be immutable", func() {
			ins := Integrations{}
			Expect(len(ins)).To(Equal(0))
			ins2 := ins.MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			})
			Expect(len(ins)).To(Equal(0))
			Expect(len(ins2)).To(Equal(1))

			in := ins2[0]

			Expect(in.CreatedAt.IsZero()).To(BeFalse())
			Expect(in.UpdatedAt.IsZero()).To(BeFalse())
		})
	})

	Context("Update", func() {
		It("should return integrations with one updated", func() {
			ins := Integrations{}
			Expect(len(ins)).To(Equal(0))

			ins = ins.MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsFacebookPixelType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			})

			Expect(len(ins)).To(Equal(3))

			in := Integration{}
			in.Id = ins[0].Id
			in.Type = MailchimpType
			in.Data = []byte("{ \"listId\": \"LIST\", \"apiKey\": \"APIKEY\" }")

			Expect(ins[0].Type).To(Equal(AnalyticsCustomType))

			ins = ins.MustUpdate(in)

			Expect(ins[0].Type).To(Equal(MailchimpType))
			Expect(ins[0].UpdatedAt).ToNot(Equal(in.UpdatedAt))
			Expect(ins[0].Mailchimp.ListId).To(Equal("LIST"))
			Expect(ins[0].Mailchimp.APIKey).To(Equal("APIKEY"))
		})

		It("should be immutable", func() {
			ins := Integrations{}
			Expect(len(ins)).To(Equal(0))

			ins = ins.MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsFacebookPixelType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			})

			Expect(len(ins)).To(Equal(3))

			in := Integration{}
			in.Id = ins[0].Id
			in.Type = MailchimpType
			in.Data = []byte("{ \"listId\": \"LIST\", \"apiKey\": \"APIKEY\" }")

			Expect(ins[0].Type).To(Equal(AnalyticsCustomType))

			ins2 := ins.MustUpdate(in)

			Expect(ins[0].Type).To(Equal(AnalyticsCustomType))
			Expect(ins[0].UpdatedAt).ToNot(Equal(ins[2].UpdatedAt))
			Expect(ins[0].Mailchimp.ListId).To(Equal(""))
			Expect(ins[0].Mailchimp.APIKey).To(Equal(""))

			Expect(ins2[0].Type).To(Equal(MailchimpType))
			Expect(ins2[0].UpdatedAt).ToNot(Equal(in.UpdatedAt))
			Expect(ins2[0].Mailchimp.ListId).To(Equal("LIST"))
			Expect(ins2[0].Mailchimp.APIKey).To(Equal("APIKEY"))
		})
	})

	Context("Remove", func() {
		It("should return return integrations with one removed", func() {
			ins := Integrations{}
			Expect(len(ins)).To(Equal(0))
			ins = ins.MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsFacebookPixelType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			})

			Expect(len(ins)).To(Equal(3))
			Expect(ins[0].Type).To(Equal(AnalyticsCustomType))
			Expect(ins[1].Type).To(Equal(AnalyticsFacebookPixelType))
			Expect(ins[2].Type).To(Equal(AnalyticsCustomType))

			ins = ins.MustRemove(ins[1].Id)

			Expect(len(ins)).To(Equal(2))
			Expect(ins[0].Type).To(Equal(AnalyticsCustomType))
			Expect(ins[1].Type).To(Equal(AnalyticsCustomType))
		})

		It("should be immutable", func() {
			ins := Integrations{}
			Expect(len(ins)).To(Equal(0))
			ins = ins.MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsFacebookPixelType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			})

			Expect(len(ins)).To(Equal(3))
			Expect(ins[0].Type).To(Equal(AnalyticsCustomType))
			Expect(ins[1].Type).To(Equal(AnalyticsFacebookPixelType))
			Expect(ins[2].Type).To(Equal(AnalyticsCustomType))

			// log.Warn("ListA")
			// for _, in := range ins {
			// 	log.Warn("%s", in.Type)
			// }

			ins2 := ins.MustRemove(ins[1].Id)
			// log.Warn("ListC")
			// for _, in := range ins {
			// 	log.Warn("%s", in.Type)
			// }
			// log.Warn("ins %s", ins)

			Expect(len(ins)).To(Equal(3))
			Expect(ins[0].Type).To(Equal(AnalyticsCustomType))
			Expect(ins[1].Type).To(Equal(AnalyticsFacebookPixelType))
			Expect(ins[2].Type).To(Equal(AnalyticsCustomType))

			Expect(len(ins2)).To(Equal(2))
			Expect(ins2[0].Type).To(Equal(AnalyticsCustomType))
			Expect(ins2[1].Type).To(Equal(AnalyticsCustomType))
		})
	})

	Context("FindById", func() {
		It("should find stuff that's there", func() {
			ins := Integrations{}
			Expect(len(ins)).To(Equal(0))

			ins = ins.MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsFacebookPixelType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			})

			Expect(len(ins)).To(Equal(3))

			in, err := ins.FindById(ins[1].Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(in.Id).To(Equal(ins[1].Id))
		})

		It("should not find stuff that's not there", func() {
			ins := Integrations{}
			Expect(len(ins)).To(Equal(0))

			ins = ins.MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsFacebookPixelType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			})

			Expect(len(ins)).To(Equal(3))

			in, err := ins.FindById("NOT HERE")
			Expect(err).To(HaveOccurred())
			Expect(in).To(BeNil())
		})
	})

	Context("FilterByType", func() {
		It("should find stuff", func() {
			ins := Integrations{}
			Expect(len(ins)).To(Equal(0))

			ins = ins.MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsFacebookPixelType,
				},
			}).MustAppend(Integration{
				BasicIntegration: BasicIntegration{
					Type: AnalyticsCustomType,
				},
			})

			Expect(len(ins)).To(Equal(3))

			results := ins.FilterByType(AnalyticsCustomType)
			Expect(len(results)).To(Equal(2))
		})
	})

	Context("Encode/Decode", func() {
		It("should encode/decode stuff", func() {
			in := Integration{}
			in.Type = MailchimpType
			in.Data = []byte("{ \"listId\": \"LIST\", \"apiKey\": \"APIKEY\" }")

			inD := Integration{}

			err := Decode(in, &inD)
			Expect(err).ToNot(HaveOccurred())
			Expect(inD.Type).To(Equal(MailchimpType))
			Expect(inD.Mailchimp.ListId).To(Equal("LIST"))
			Expect(inD.Mailchimp.APIKey).To(Equal("APIKEY"))

			inE := Integration{}

			r, _ := regexp.Compile("\\s")

			err = Encode(inD, &inE)
			Expect(err).ToNot(HaveOccurred())
			Expect(inE.Type).To(Equal(MailchimpType))
			Expect(r.ReplaceAllString(string(inE.Data), "")).To(Equal("{\"listId\":\"LIST\",\"apiKey\":\"APIKEY\"}"))
		})
	})
})
