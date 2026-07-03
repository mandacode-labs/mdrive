package txmanager

import (
	"context"
	"github.com/mandacode-labs/mdrive/ent"
)

type entTxManager struct {
	client *ent.Client
}

// WithTx implements [TxManager].
func (e *entTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*ent.Tx); ok {
		// Already in a transaction, just call the function with the existing context.
		return fn(ctx)
	}

	tx, err := e.client.Tx(ctx)
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

func NewEntTxManager(client *ent.Client) TxManager {
	return &entTxManager{client: client}
}

// EntTxFromContext retrieves the ent.Tx from the context, if present.
// If the context does not contain an ent.Tx, it returns nil and false.
func EntTxFromContext(ctx context.Context) (*ent.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*ent.Tx)
	return tx, ok
}
