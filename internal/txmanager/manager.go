package txmanager

import "context"

type txKey struct{}

type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
