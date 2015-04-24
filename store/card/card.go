package card

import "github.com/gin-gonic/gin"

func GetCard(c *gin.Context) {
	// email, _ := auth.GetEmail(c)

	// db := datastore.New(c)
	// ctx := middleware.GetAppEngine(c)

	// if count, err := db.Query("order").Filter("Email =", email).Count(ctx); count < 1 || err != nil {
	// 	c.Fail(404, errors.New("No orders found for that user."))
	// 	return
	// }

	// user, _ := auth.GetUser(c)

	// template.Render(c, "skullycard.html",
	// 	"user", user,
	// 	"GCSBucket", config.Google.Bucket.ImageUploads,
	// 	"GCSAPIKey", config.Google.APIKey)
}
