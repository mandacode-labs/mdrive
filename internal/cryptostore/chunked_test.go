package cryptostore

import (
	"bytes"
	"testing"

	"github.com/mandacode-labs/mdrive/internal/crypto"
)

func TestChunkedRoundTrip(t *testing.T) {
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i)
	}
	provider, err := crypto.NewDEKProvider(master)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := provider.NewWrappedDEK()
	if err != nil {
		t.Fatal(err)
	}
	dek, err := provider.Unwrap(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := crypto.NewNodeCipher(dek)
	if err != nil {
		t.Fatal(err)
	}
	aad := []byte("d1:b:drives/d1/uploads/u1")

	for _, size := range []int{0, 1, 100, 1024, 65535, 65536, 65537, 200000} {
		plain := make([]byte, size)
		for i := range plain {
			plain[i] = byte(i % 251)
		}
		var ctBuf bytes.Buffer
		if _, err := encodeStreaming(cipher, bytes.NewReader(plain), &ctBuf, aad); err != nil {
			t.Fatalf("encode size=%d: %v", size, err)
		}
		var ptBuf bytes.Buffer
		if err := decodeStreaming(cipher, bytes.NewReader(ctBuf.Bytes()), &ptBuf, aad); err != nil {
			t.Fatalf("decode size=%d: %v", size, err)
		}
		if !bytes.Equal(ptBuf.Bytes(), plain) {
			t.Fatalf("size=%d: round-trip mismatch (got %d bytes, want %d)", size, ptBuf.Len(), size)
		}
	}
}
