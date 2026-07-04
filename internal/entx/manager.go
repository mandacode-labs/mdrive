package entx

import (
	"context"
	"github.com/mandacode-labs/mdrive/ent"
)

type txKey struct{}

type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type txManager struct {
	client *ent.Client
}

// WithTx implements [TxManager].
func (t *txManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*ent.Tx); ok {
		// Already in a transaction, just call the function with the existing context.
		return fn(ctx)
	}

	tx, err := t.client.Tx(ctx)
	if err != nil {
		return err
	}

	ctxWithTx := context.WithValue(ctx, txKey{}, tx)

	err = fn(ctxWithTx)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return rollbackErr
		}
		return err
	}

	return tx.Commit()
}

func NewTxManager(client *ent.Client) TxManager {
	return &txManager{client: client}
}

// FromContext retrieves the ent.Tx from the context, if present.
// If the context does not contain an ent.Tx, it returns nil and false.
func FromContext(ctx context.Context) (*ent.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*ent.Tx)
	return tx, ok
}
