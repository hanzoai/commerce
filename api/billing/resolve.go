package billing

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/account"
)

// Who pays is not commerce's opinion. It is the credential's own signed statement,
// read by the ONE rule every layer that touches money shares — hanzoai/account.
//
// The account money lands in and the gate reads out of MUST be the same account,
// or a customer tops up one wallet and spends from another. commerce (the grant,
// the top-up, the balance read) and ai (the gate, the usage debit) are separate
// repos that never call each other, so the only thing that can hold them together
// is calling the same function on the same fact. They both call account.Payer.
//
// This file used to MIRROR ai's rule from a copy, keyed on two env allowlists
// (PERSONAL_BILLING_ORGS / ORG_BILLING_ORGS). A copy is not a shared rule: ai
// moved to the signed claim and this copy did not, so "the two can never drift"
// became false the moment it was written. Worse, the allowlists were armable —
// setting ORG_BILLING_ORGS=hanzo (a documented billing tourniquet) would have made
// commerce credit "hanzo" while ai debited "hanzo/alice", funding one account and
// 402'ing off the other. The env is gone; the tourniquet cannot be armed.
//
// An Account is owned by a Person, an Org, or a Project, and the credential names
// which one it pays from — that IS attaching an account to a user, an org, or a
// project, stated once, at the identity boundary, over IAM's signature
// (iam/object/billing_account.go). There is no second attachment mechanism.

// payer returns the Account this request pays from, resolved from the caller's
// gateway-minted identity headers. It is the single seam between commerce's
// request context and the shared rule.
//
// The headers it reads are all server-minted and stripped from client input at the
// edge (gateway/iamauth MintedIdentityHeaders), so none is forgeable:
//
//	org      — the resolved org (middleware), the ledger the money lives in
//	X-User-Id            — the caller, the name half of a person's account
//	X-Billing-Account-Id — the `billing_account` claim: the account the credential
//	                       NAMES, minted by the gateway from the validated JWT
//
// A machine credential is resolved from the claim, which IAM derives from the
// client_credentials GRANT SHAPE. commerce cannot see IAM's User.Type and does not
// guess at it: a pre-claim machine token therefore falls back to the person branch
// here while ai's falls back to the org pool. That divergence is inherited, not
// introduced — it is exactly what the two copies did before — and the claim closes
// it for every token minted since. It deletes when the last pre-claim token expires.
func payer(c *gin.Context) account.Account {
	org := orgBillingKey(c)
	if org == "" {
		return account.Account{} // no org resolved — caller cannot bill; handlers 401
	}
	return account.Payer(account.Credential{
		Owner:   org,
		Name:    c.GetHeader("X-User-Id"),
		Account: c.GetHeader("X-Billing-Account-Id"),
	})
}
