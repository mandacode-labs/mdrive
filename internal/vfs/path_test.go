package vfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "/"},
		{"/", "/"},
		{"/a/b/c", "/a/b/c"},
		{"/a//b///c", "/a/b/c"},
		{"/a/./b", "/a/b"},
		{"/a/b/..", "/a"},
		{"/a/b/../c", "/a/c"},
		{"/..", "/"},    // cannot ascend above root
		{"/../a", "/a"}, // path.Clean collapses
		{"  /a/b  ", "/a/b"},
		{"/a/b/", "/a/b"},
		{"a/b", "/a/b"}, // path.Clean of "/a/b" is "/a/b"
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, cleanPath(tt.in))
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"/", nil},
		{"", nil},
		{"/a/b/c", []string{"a", "b", "c"}},
		{"/a", []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, splitPath(tt.in))
		})
	}
}
