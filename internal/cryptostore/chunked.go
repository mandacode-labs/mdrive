// Package cryptostore wraps a vfs.Store with envelope encryption for
// object bodies. The wrapper implements the vfs.Store interface; all
// byte-producing calls (PutObject, GetObject) encrypt/decrypt on the
// fly using the per-drive DEK. AAD binds the ciphertext to the
// (driveID, bucket, key) triple, preventing swap attacks.
package cryptostore

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/mandacode-labs/mdrive/internal/crypto"
)

const (
	chunkSize     = 64 * 1024
	nonceSize     = 12
	sizeSize      = 8
	sentinelIndex = uint32(0xFFFFFFFF)
	indexSize     = 4
)

// encodeStreaming reads plaintext from r, encrypts it with cipher using
// the streaming chunked AEAD construction, and writes the ciphertext
// to w. The output layout is:
//
//	[12-byte base nonce] [8-byte plaintext size BE] [chunk 0 ct] ... [final chunk ct]
//
// Each chunk is `chunkSize` bytes of plaintext (the last is shorter)
// sealed with AES-GCM. Per-chunk nonce is derived from the base nonce
// by XOR with the chunk index encoded into the last 4 bytes of the
// 12-byte nonce; the final chunk uses index 0xFFFFFFFF as a sentinel
// so the decryptor knows when to stop. AAD for every chunk is aad.
// The plaintext size is encoded in the header so the decoder knows
// the total plaintext length without out-of-band metadata.
func encodeStreaming(cipher *crypto.NodeCipher, r io.Reader, w io.Writer, aad []byte) (int64, error) {
	baseNonce, err := crypto.RandomBytes(nonceSize)
	if err != nil {
		return 0, fmt.Errorf("cryptostore: generate nonce: %w", err)
	}
	if _, err := w.Write(baseNonce); err != nil {
		return 0, fmt.Errorf("cryptostore: write nonce: %w", err)
	}
	// Buffer the plaintext so we can compute the size before
	// committing the header. For very large objects this
	// duplicates the memory footprint; a length-prefix scheme
	// would let us stream without buffering. We accept the
	// memory cost for simplicity in this revision.
	plaintext, err := io.ReadAll(r)
	if err != nil {
		return 0, fmt.Errorf("cryptostore: read plaintext: %w", err)
	}
	if err := writeSize(w, int64(len(plaintext))); err != nil {
		return 0, err
	}
	if len(plaintext) == 0 {
		// Empty plaintext: emit an empty final chunk so the
		// decoder sees the sentinel and authenticates a 0-byte
		// message.
		chunkNonce := deriveNonce(baseNonce, sentinelIndex)
		ct := cipher.SealWithNonce(chunkNonce, nil, aad)
		if _, err := w.Write(ct); err != nil {
			return 0, fmt.Errorf("cryptostore: write final chunk: %w", err)
		}
		return 0, nil
	}
	totalChunks := (int64(len(plaintext)) + int64(chunkSize) - 1) / int64(chunkSize)
	for i := int64(0); i < totalChunks; i++ {
		offset := int(i) * chunkSize
		end := offset + chunkSize
		if end > len(plaintext) {
			end = len(plaintext)
		}
		pt := plaintext[offset:end]
		idx := uint32(i)
		if i == totalChunks-1 {
			idx = sentinelIndex
		}
		chunkNonce := deriveNonce(baseNonce, idx)
		ct := cipher.SealWithNonce(chunkNonce, pt, aad)
		if _, err := w.Write(ct); err != nil {
			return 0, fmt.Errorf("cryptostore: write chunk: %w", err)
		}
	}
	return int64(len(plaintext)), nil
}

// decodeStreaming reads the streaming AEAD ciphertext from r, decrypts
// it, and writes plaintext to w. Returns an error on AAD mismatch or
// any chunk authentication failure. The plaintext size is read from
// the stream header; a size mismatch (e.g. truncated stream) surfaces
// as an error.
func decodeStreaming(cipher *crypto.NodeCipher, r io.Reader, w io.Writer, aad []byte) error {
	baseNonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(r, baseNonce); err != nil {
		return fmt.Errorf("cryptostore: read nonce: %w", err)
	}
	plaintextSize, err := readSize(r)
	if err != nil {
		return fmt.Errorf("cryptostore: read size: %w", err)
	}
	tagSize := cipher.TagSize()
	if plaintextSize == 0 {
		ct := make([]byte, tagSize)
		if _, err := io.ReadFull(r, ct); err != nil {
			return fmt.Errorf("cryptostore: read empty sentinel: %w", err)
		}
		chunkNonce := deriveNonce(baseNonce, sentinelIndex)
		if _, err := cipher.OpenWithNonce(chunkNonce, ct, aad); err != nil {
			return fmt.Errorf("cryptostore: decrypt empty: %w", err)
		}
		return nil
	}
	totalChunks := (plaintextSize + int64(chunkSize) - 1) / int64(chunkSize)
	for i := int64(0); i < totalChunks; i++ {
		plainChunk := int64(chunkSize)
		if i == totalChunks-1 {
			plainChunk = plaintextSize - i*int64(chunkSize)
		}
		ct := make([]byte, plainChunk+int64(tagSize))
		n, rerr := io.ReadFull(r, ct)
		if rerr != nil && rerr != io.EOF && rerr != io.ErrUnexpectedEOF {
			return fmt.Errorf("cryptostore: read chunk %d: %w", i, rerr)
		}
		ct = ct[:n]
		idx := uint32(i)
		if i == totalChunks-1 {
			idx = sentinelIndex
		}
		chunkNonce := deriveNonce(baseNonce, idx)
		pt, err := cipher.OpenWithNonce(chunkNonce, ct, aad)
		if err != nil {
			return fmt.Errorf("cryptostore: decrypt chunk %d: %w", i, err)
		}
		if _, err := w.Write(pt); err != nil {
			return fmt.Errorf("cryptostore: write plaintext: %w", err)
		}
	}
	return nil
}

func writeSize(w io.Writer, size int64) error {
	var buf [sizeSize]byte
	binary.BigEndian.PutUint64(buf[:], uint64(size))
	_, err := w.Write(buf[:])
	return err
}

func readSize(r io.Reader) (int64, error) {
	var buf [sizeSize]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(buf[:])), nil
}

func deriveNonce(base []byte, index uint32) []byte {
	out := make([]byte, nonceSize)
	copy(out, base)
	off := nonceSize - indexSize
	binary.BigEndian.PutUint32(out[off:], binary.BigEndian.Uint32(out[off:])^index)
	return out
}
