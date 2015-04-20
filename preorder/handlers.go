package preorder

import (
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"
	"crowdstart.io/util/template"

	salesforce "crowdstart.io/thirdparty/salesforce/tasks"
)

func getOrderKey(db *datastore.Datastore, id string) (datastore.Key, error) {
	// get intid
	intid, err := strconv.Atoi(id)

	if err != nil {
		return nil, err
	}

	// Get orders by id
	key := db.NewKey("order", "", int64(intid), nil)
	return key, nil
}

// GET /order/:id
func GetPreorder(c *gin.Context) {
	db := datastore.New(c)
	ctx := db.Context

	id := c.Params.ByName("id")

	// Make sure we don't have a token id.
	count, _ := db.Query("invite-token").Filter("Id=", id).Count(ctx)
	if count > 0 {
		c.Redirect(302, "../expired-token")
		return
	}

	order := new(models.Order)
	key, err := getOrderKey(db, id)
	if err != nil {
		log.Panic("Error retrieving order id associated with the user's email", err)
	}

	err = db.Get(key, order)

	// Query will not error when number of entities returned by query is zero.
	// We continue based on the assumption that when saving that will create an actual order.
	if err != nil {
		log.Panic("Error retrieving orders associated with the user's email", err)
	}

	// Get user from order
	user := new(models.User)
	if err := db.Get(order.UserId, user); err != nil {
		log.Error("Failed to fetch user: %v", err, c)
		// Bad token
		c.Redirect(302, "../")
		return
	}

	// Load order
	order.LoadVariantsProducts(c)

	// Order id
	contributionId := strconv.Itoa(int(key.IntID()))

	// Create  JSON
	contributionsJSON := "{}"
	orderJSON := json.Encode(order)
	userJSON := json.Encode(user)

	// Find all of a user's contributions
	var contributions []models.Contribution
	db.Query("contribution").Filter("Id =", contributionId).GetAll(db.Context, &contributions)
	if len(contributions) > 0 {
		log.Debug("Contributions: %v", contributions)
		contributionsJSON = json.Encode(contributions)
	}

	// Get all products
	var products []models.Product
	db.Query("product").GetAll(db.Context, &products)

	// Create map of slug -> product
	productsMap := make(map[string]models.Product)
	for _, product := range products {
		productsMap[product.Slug] = product
	}
	productsJSON := json.Encode(productsMap)

	template.Render(c, "preorder.html",
		"tokenId", "",
		"user", user,
		"productsJSON", productsJSON,
		"contributionsJSON", contributionsJSON,
		"orderJSON", orderJSON,
		"contributionId", contributionId,
		"orderId", order.Id,
		"userJSON", userJSON,
		"items", order.Items,
	)
}

// POST /order/save
func SavePreorder(c *gin.Context) {
	form := new(PreorderForm)
	if err := form.Parse(c); err != nil {
		c.Fail(500, err)
		return
	}

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)
	q := queries.New(ctx)

	// Get user from datastore
	user := new(models.User)
	if err := q.GetUserByEmail(form.User.Email, user); err != nil {
		c.Fail(500, errors.New("Failed to find user."))
		return
	}

	// Ensure that token matches email
	// tokens := getTokens(c, user.Id)
	// if len(tokens) < 1 {
	// 	c.Fail(500, errors.New("Failed to find pre-order token."))
	// 	return
	// } else if !hasToken(tokens, form.Token.Id) {
	// 	c.Fail(500, errors.New("Token not valid for user email."))
	// 	return
	// }

	// log.Debug("Found token")

	// Update user's password if this is the first time saving.

	// if !user.HasPassword() {
	// 	user.PasswordHash = form.User.PasswordHash
	// }

	// Update user information
	user.Phone = form.User.Phone
	user.FirstName = form.User.FirstName
	user.LastName = form.User.LastName
	user.ShippingAddress = form.ShippingAddress
	log.Debug("User: %v", user)

	var order models.Order
	if form.Order.Id != "" {
		key, err := db.DecodeKey(form.Order.Id)
		if err = db.Get(key, &order); err != nil {
			log.Error("Invalid Order.Id: %v", err, ctx)
			c.Fail(500, err)
			return
		}
		if err := db.Get(key, &order); err != nil {
			order = form.Order
		}
	} else {
		order = form.Order
	}
	order.UpdatedAt = time.Now()
	order.ShippingAddress = form.ShippingAddress
	log.Debug("ShippingAddress: %v", user)

	// TODO: Optimize this, multiget, use caching.
	for i, lineItem := range form.Order.Items {
		log.Debug("Fetching variant for %v", lineItem.SKU())

		// Fetch Variant for LineItem from datastore
		if err := db.GetKind("variant", lineItem.SKU(), &lineItem.Variant); err != nil {
			log.Error("Failed to find variant for: %v", lineItem.SKU(), ctx)
			c.Fail(500, err)
			return
		}

		// Fetch Product for LineItem from datastore
		if err := db.GetKind("product", lineItem.Slug(), &lineItem.Product); err != nil {
			log.Error("Failed to find product for: %v", lineItem.Slug(), ctx)
			c.Fail(500, err)
			return
		}

		// Set SKU so we can deserialize later
		lineItem.SKU_ = lineItem.SKU()
		lineItem.Slug_ = lineItem.Slug()

		// Update item in order
		order.Items[i] = lineItem

		// Update subtotal
		order.Subtotal += lineItem.Price()
	}

	// Update Total
	order.Total = order.Subtotal + order.Shipping + order.Tax
	order.UserId = user.Id

	// Save order
	log.Debug("Saving order: %v", order)
	if order.Id != "" {
		log.Debug("Using OrderId: %v", order.Id)
		key, err := db.DecodeKey(order.Id)
		if err != nil {
			log.Error("Invalid Order.Id: %v", err, ctx)
			c.Fail(500, err)
			return
		}

		// Retrieve existing order and update things we care about
		if _, err := db.Put(key, &order); err != nil {
			log.Error("Error saving order: %v", err, ctx)
			c.Fail(500, err)
			return
		}
	} else {
		log.Debug("No order Id found")
		key := db.AllocateIntKey("order")
		if _, err := db.Put(key, &order); err != nil {
			log.Error("Error saving order: %v", err, ctx)
			c.Fail(500, err)
			return
		}
	}

	// Save user back to database
	if err := q.UpsertUser(user); err != nil {
		log.Error("Error saving user information", err, ctx)
		c.Fail(500, err)
		return
	}

	// Look up campaign to see if we need to sync with salesforce
	campaign := models.Campaign{}
	if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
		log.Error(err, c)
	}

	log.Debug("Synchronize with salesforce if '%v' != ''", campaign.Salesforce.AccessToken)
	if campaign.Salesforce.AccessToken != "" {
		salesforce.CallUpsertUserTask(db.Context, &campaign, user)
		salesforce.CallUpsertOrderTask(db.Context, &campaign, &order)
	}

	// TODO: Reenable for production!
	// mandrill.SendTransactional.Call(ctx, "email/preorder-updated.html",
	// 	user.Email,
	// 	user.Name(),
	// 	"SKULLY preorder information updated")

	c.Redirect(302, config.UrlFor("preorder", "/thanks"))
}

// GET /login
func Login(c *gin.Context) {
	template.Render(c, "login.html")
}

// POST /login
func LoginSubmit(c *gin.Context) {
	query := c.Request.URL.Query()
	redirectUrl := query.Get("redirect-url")

	// Parse login form
	f := new(auth.LoginForm)
	if err := f.Parse(c); err != nil {
		template.Render(c, "login.html", "message", "The email or password you entered is incorrect.")
		return
	}

	// Verify password
	err := auth.VerifyUser(c)
	if err != nil {
		template.Render(c, "login.html", "message", "The email or password you entered is incorrect.")
		return
	}

	c.Redirect(302, redirectUrl)
}

// hasToken checks whether any of the tokens have the id
func hasToken(tokens []models.Token, id string) bool {
	for _, token := range tokens {
		if token.Id == id {
			return true
		}
	}
	return false
}

func getTokens(c *gin.Context, userId string) []models.Token {
	db := datastore.New(c)

	// Look up tokens for this user
	log.Debug("Searching for valid token for: %v", userId, c)

	tokens := make([]models.Token, 0)
	if _, err := db.Query("invite-token").Filter("UserId =", userId).GetAll(db.Context, &tokens); err != nil {
		log.Panic("Failed to query for tokens: %v", err, c)
	}

	return tokens
}
