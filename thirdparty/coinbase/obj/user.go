package obj

type User struct {
	Id               string // resource ID
	Name             string // User's public name - optional
	Username         string // Payment method's native currency
	Profile_Location string // Location for user's public profile
	Profile_Bio      string // Bio for user's public profile
	Profile_Url      string // Public profile location if a user has one
	Avatar_Url       string // User's avatar url
	Resource         string
	Resource_Path    string
}
