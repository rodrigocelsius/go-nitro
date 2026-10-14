package keycloak

import "github.com/rodrigocelsius/go-nitro/domain/v2"

func domainUnauthorized(msg string) error {
	return &domain.UnauthorizedError{Message: msg}
}
