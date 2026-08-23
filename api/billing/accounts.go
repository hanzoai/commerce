package billing

import (
	"context"
	"errors"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/json/http"
)

// Account is a billing account. In Commerce an organization IS the account, so
// the id, name and age are the org's, and the currency is the usd this surface
// has always reported.
type Account struct {
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	OrgId     string    `json:"orgId"`
	OrgName   string    `json:"orgName"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"createdAt"`
	// Role is the caller's standing in the org. It is absent when no caller was
	// named, so an anonymous read gets an account rather than an implied
	// membership.
	Role string `json:"role,omitempty"`
}

// Member is one member of a billing account. The roster lives in IAM, not in
// Commerce, so the only member Commerce can name is the caller itself.
type Member struct {
	Id      string    `json:"id"`
	UserId  string    `json:"userId"`
	Email   string    `json:"email"`
	Role    string    `json:"role"`
	AddedAt time.Time `json:"addedAt"`
}

// errForeignAccount is the roster refusal: the account named belongs to another
// organization.
var errForeignAccount = errors.New("members: account belongs to another organization")

// IsAccountRefusal reports whether err is that refusal. ListAccountMembers
// renders it as its 403, and a caller that reached the core another way reads
// the same answer here rather than guessing from an error string.
func IsAccountRefusal(err error) bool { return errors.Is(err, errForeignAccount) }

// ListAccounts is the billing accounts an org exposes — one, itself.
//
// The caller's identity arrives as values because the caller is not always a
// request: a peer holding no HTTP context asks the same question over the
// internal plane and must get the same account back. An empty subject means
// nobody was named, and then the account carries no role.
func ListAccounts(ctx context.Context, org *organization.Organization, subject, role string) ([]Account, error) {
	if org == nil {
		return nil, errors.New("accounts: no organization")
	}
	a := Account{
		Id:        org.Id(),
		Name:      org.FullName,
		OrgId:     org.Id(),
		OrgName:   org.Name,
		Currency:  "usd",
		CreatedAt: org.CreatedAt,
	}
	if subject != "" {
		a.Role = role
	}
	return []Account{a}, nil
}

// ListMembers is the roster of one billing account: the caller, or nobody when
// no caller was named.
//
// It takes the account id and the caller's identity as values so the check that
// guards the roster — this account is yours — holds for every asker, not only
// for one arriving over HTTP. A foreign account id is errForeignAccount here,
// which the endpoint renders as its refusal.
func ListMembers(ctx context.Context, org *organization.Organization, accountId, subject, email, role string) ([]Member, error) {
	if org == nil {
		return nil, errors.New("members: no organization")
	}
	if accountId != org.Id() {
		return nil, errForeignAccount
	}
	members := make([]Member, 0)
	if subject != "" {
		members = append(members, Member{
			Id:      subject,
			UserId:  subject,
			Email:   email,
			Role:    role,
			AddedAt: org.CreatedAt,
		})
	}
	return members, nil
}

// ListBillingAccounts returns billing accounts visible to the caller.
// In Commerce each organization is one billing account. The authenticated
// org is returned as the single account for the current token.
//
//	GET /v1/billing/accounts
func ListBillingAccounts(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	// Surface the caller's role if the gateway authenticated them.
	// claims is always non-nil; an empty Subject means anonymous and
	// we leave the "role" field unset rather than implying membership.
	var subject, role string
	if claims := iammiddleware.GetIAMClaims(c); claims.Subject != "" {
		subject, role = claims.Subject, "member"
		for _, r := range claims.Roles {
			if r == "admin" || r == "owner" {
				role = r
				break
			}
		}
	}

	accounts, err := ListAccounts(c.Context(), org, subject, role)
	if err != nil {
		return http.Fail(c, 500, "failed to list billing accounts", err)
	}

	return c.JSON(200, accounts)
}

// CreateBillingAccount is a no-op stub. Billing accounts are provisioned via
// IAM/console org creation; Commerce does not manage org lifecycle.
// Returns 501 to signal the caller to redirect to the org provisioning flow.
//
//	POST /v1/billing/accounts
func CreateBillingAccount(c *zip.Ctx) error {
	return http.Fail(c, 501, "billing account creation must be done via the Hanzo console", nil)
}

// ListAccountMembers returns the members of a billing account (org).
// Currently returns the requesting IAM user as the sole member, since
// Commerce does not store a full membership roster (that lives in IAM).
//
//	GET /v1/billing/accounts/:id/members
func ListAccountMembers(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	// claims is always non-nil; an empty Subject means anonymous and
	// the response stays an empty members list rather than synthesizing
	// a phantom row.
	var subject, email, role string
	if claims := iammiddleware.GetIAMClaims(c); claims.Subject != "" {
		subject, email, role = claims.Subject, claims.Email, "member"
		for _, r := range claims.Roles {
			if r == "admin" || r == "owner" {
				role = r
				break
			}
		}
	}

	members, err := ListMembers(c.Context(), org, c.Param("id"), subject, email, role)
	if err != nil {
		return http.Fail(c, 403, "access denied to billing account", nil)
	}

	return c.JSON(200, members)
}

// AddAccountMember is a stub. Member management is done via IAM.
//
//	POST /v1/billing/accounts/:id/members
func AddAccountMember(c *zip.Ctx) error {
	return http.Fail(c, 501, "member management must be done via the Hanzo console", nil)
}

// UpdateMemberRole is a stub. Role updates are done via IAM.
//
//	PATCH /v1/billing/accounts/:id/members/:memberId
func UpdateMemberRole(c *zip.Ctx) error {
	return http.Fail(c, 501, "role updates must be done via the Hanzo console", nil)
}

// RemoveAccountMember is a stub. Member removal is done via IAM.
//
//	DELETE /v1/billing/accounts/:id/members/:memberId
func RemoveAccountMember(c *zip.Ctx) error {
	return http.Fail(c, 501, "member removal must be done via the Hanzo console", nil)
}
