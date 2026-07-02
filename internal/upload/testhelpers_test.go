package upload

import (
	"testing"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

func assertKind(t *testing.T, err error, want errorx.Kind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error of kind %s, got nil", want)
	}
	if errorx.KindOf(err) != want {
		t.Fatalf("expected kind %s, got %s (err=%v)", want, errorx.KindOf(err), err)
	}
}
