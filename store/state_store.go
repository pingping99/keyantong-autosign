package store

import "keyantong/domain"

// StateStore handles sign state persistence.
type StateStore interface {
	Load() (*domain.SignState, error)
	Save(state *domain.SignState) error
}
