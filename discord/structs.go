package discord

import "github.com/headd16/goself/types"

// Selfbot holds the account token and, after READY, the logged-in user.
// Prefer gateway.User() to read the user from other goroutines.
type Selfbot struct {
	Token string
	User  types.User
}
