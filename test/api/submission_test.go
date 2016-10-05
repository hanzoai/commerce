package test

import (
	"crowdstart.com/models/submission"
	"crowdstart.com/models/user"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("submission", func() {
	Context("New submission", func() {
		req := new(submission.Submission)
		res := new(submission.Submission)

		Before(func() {
			usr := user.Fake(db)
			usr.MustCreate()
			req = submission.Fake(db, usr.Id())
			res = submission.New(db)

			// Create new submission
			cl.Post("/submission", req, res)
		})

		It("Should create new submissions", func() {
			Expect(res.Email).To(Equal(req.Email))
			Expect(res.UserId).To(Equal(req.UserId))
		})
	})
	Context("Get submission", func() {
		req := new(submission.Submission)
		res := new(submission.Submission)

		Before(func() {
			usr := user.Fake(db)
			usr.MustCreate()
			req = submission.Fake(db, usr.Id())
			req.MustCreate()

			res = submission.New(db)

			cl.Get("/submission/"+req.Id(), res)
		})

		It("Should get submissions", func() {
			Expect(res.Email).To(Equal(req.Email))
			Expect(res.UserId).To(Equal(req.UserId))
		})
	})
	Context("Delete submission", func() {
		res := ""

		Before(func() {
			usr := user.Fake(db)
			usr.MustCreate()
			req := submission.Fake(db, usr.Id())
			req.MustCreate()

			cl.Delete("/submission/" + req.Id())
			res = req.Id()

		})

		It("Should delete submissions", func() {
			sub := submission.New(db)
			err := sub.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})
})
