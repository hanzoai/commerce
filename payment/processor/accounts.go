package processor

import "sync"

// Account is a processor's account as the routing policy gives it: plain settings
// (environment, location, public keys) and secrets already read from KMS. The policy
// holds only the KMS paths; the values live here, in memory, from the moment the policy
// is applied, and a provider reads its configuration from them and from nothing else.
type Account struct {
	ID       string
	Settings map[string]string
	Secrets  map[string]string
}

// Setting is the named setting, or "".
func (a Account) Setting(name string) string { return a.Settings[name] }

// Secret is the named secret, or "".
func (a Account) Secret(name string) string { return a.Secrets[name] }

// Reconfigurable is a processor that reads its configuration from its accounts.
type Reconfigurable interface{ Reconfigure() }

var (
	accountsMu sync.RWMutex
	accounts   = map[ProcessorType][]Account{}
)

// SetAccounts makes accts the accounts of processor t, replacing what it had, and tells the
// registered processor to read them. It returns what puts the previous accounts back.
func SetAccounts(t ProcessorType, accts []Account) (restore func()) {
	accountsMu.Lock()
	old := accounts[t]
	if len(accts) == 0 {
		delete(accounts, t)
	} else {
		accounts[t] = accts
	}
	accountsMu.Unlock()
	reconfigure(t)
	return func() {
		accountsMu.Lock()
		if len(old) == 0 {
			delete(accounts, t)
		} else {
			accounts[t] = old
		}
		accountsMu.Unlock()
		reconfigure(t)
	}
}

// AccountOf is processor t's account with the given id, or the zero Account.
func AccountOf(t ProcessorType, id string) Account {
	accountsMu.RLock()
	defer accountsMu.RUnlock()
	for _, a := range accounts[t] {
		if a.ID == id {
			return a
		}
	}
	return Account{}
}

// FirstAccount is processor t's first account (the policy lists only accounts that are on).
func FirstAccount(t ProcessorType) (Account, bool) {
	accountsMu.RLock()
	defer accountsMu.RUnlock()
	if l := accounts[t]; len(l) > 0 {
		return l[0], true
	}
	return Account{}, false
}

func reconfigure(t ProcessorType) {
	if p, err := Global().Registered(t); err == nil {
		if r, ok := p.(Reconfigurable); ok {
			r.Reconfigure()
		}
	}
}
