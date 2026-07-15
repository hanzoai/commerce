package review

import (
	"errors"
	"strconv"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/review"
	"github.com/hanzoai/commerce/thirdparty/recaptcha"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func forced404(c *zip.Ctx) error {
	return http.Fail(c, 404, "Review does not exist", nil)
}

func get(r *rest.Rest) func(c *zip.Ctx) error {
	return func(c *zip.Ctx) error {
		if !r.CheckPermissions(c, "get") {
			return nil
		}

		id := c.Param(r.ParamId)

		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c.Context()))

		rev := review.New(db)
		if err := rev.GetById(id); err != nil {
			return http.Fail(c, 400, "Failed to query review", err)
		}

		if !rev.Enabled {
			return http.Fail(c, 404, "Review does not exist", nil)
		}

		return http.Render(c, 200, rev)
	}
}

func list(r *rest.Rest) func(c *zip.Ctx) error {
	return func(c *zip.Ctx) error {
		if !r.CheckPermissions(c, "list") {
			return nil
		}

		// Determine deafult sort order
		sortField := c.Query("sort")
		if sortField == "" {
			sortField = r.DefaultSortField
		}

		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c.Context()))

		// Create query
		q := review.Query(db).Order(sortField).Filter("Enabled=", true)

		// Update query with page/display params
		var display int
		var err error
		pageStr := c.Query("page")
		displayStr := c.Query("display")
		limitStr := c.Query("limit")

		// if we have pagination values, then trigger pagination calculations
		if displayStr != "" {
			if display, err = strconv.Atoi(displayStr); err == nil && display > 0 {
				q = q.Limit(display)
			} else {
				return r.Fail(c, 500, "'display' must be positive and non-zero.", err)
			}
		}

		if pageStr != "" && displayStr != "" {
			if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
				q = q.Offset(display * (page - 1))
			} else {
				return r.Fail(c, 500, "'page' must be positive and non-zero.", err)
			}
		}

		var revs []review.Review
		if _, err = q.GetAll(&revs); err != nil {
			return r.Fail(c, 500, "Failed to list "+r.Kind, err)
		}

		count, err := q.Count()
		if err != nil {
			return r.Fail(c, 500, "Could not count the models.", err)
		}

		if limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
				count = limit
			}
		}

		return r.Render(c, 200, rest.Pagination{
			Page:    pageStr,
			Display: displayStr,
			Models:  revs,
			Count:   count,
		})
	}
}

type createReq struct {
	*review.Review
	Captcha string `json:"g-recaptcha-response"`
}

func post(r *rest.Rest) func(c *zip.Ctx) error {
	return func(c *zip.Ctx) error {
		if !r.CheckPermissions(c, "create") {
			return nil
		}

		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c.Context()))

		req := &createReq{}
		req.Review = review.New(db)

		rev := req.Review

		if err := json.DecodeBytes(c.Body(), &req); err != nil {
			return r.Fail(c, 400, "Failed decode request body", err)
		}

		if org.Recaptcha.Enabled && !recaptcha.Challenge(db.Context, org.Recaptcha.SecretKey, req.Captcha) {
			return http.Fail(c, 400, "Captcha needs to be completed", errors.New("Captcha needs to be completed"))
		}

		if err := rev.Create(); err != nil {
			return r.Fail(c, 500, "Failed to create "+r.Kind, err)
		}
		c.SetHeader("Location", c.Path()+"/"+rev.Id())
		return r.Render(c, 201, rev)
	}
}

func Route(router zip.Router, args ...zip.Handler) {
	api := rest.New(review.Review{})

	api.Update = forced404
	api.Patch = forced404
	api.Get = get(api)
	api.List = list(api)
	api.Create = post(api)

	api.Route(router, args...)
}
