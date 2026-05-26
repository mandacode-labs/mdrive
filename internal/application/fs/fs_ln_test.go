package fs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func TestLn_EmptyTarget(t *testing.T) {
	svc := NewService(nil, nil, nil, nil)

	_, err := svc.Ln(context.Background(), "sys", "/link", "")
	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestLn_TargetTooLong(t *testing.T) {
	svc := NewService(nil, nil, nil, nil)

	target := make([]byte, 4097)
	for i := range target {
		target[i] = 'a'
	}

	_, err := svc.Ln(context.Background(), "sys", "/link", string(target))
	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}
