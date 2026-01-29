package store

import "keyantong/domain"

// StateStore handles sign state persistence.
type StateStore interface {
	Load(accountID string) (*domain.SignState, error)
	Save(accountID string, state *domain.SignState) error
}
