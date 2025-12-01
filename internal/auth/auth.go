package auth

import (
	"net/http"
)

type Principal struct {
	ID    string
	Scope string
}

type Authenticator interface {
	Authenticate(r *http.Request) (Principal, error)
}
