package idempotent

import "context"

type IdempotencyService interface {
	Exists(ctx context.Context, keys string) (bool, error)
	MExists(ctx context.Context, key ...string) ([]bool, error)
}
