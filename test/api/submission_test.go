package test

import (
	"crowdstart.com/models/submission"
	"crowdstart.com/models/user"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("submission", func() {
	Context("New submission", func() {
		var req *submission.Submission
		var res *submission.Submission

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
})
