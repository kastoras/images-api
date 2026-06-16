package files

import (
	"errors"
	"io"
	"testing"
)

type stubFile struct {
	closeErr error
}

func (s *stubFile) Read(_ []byte) (int, error)            { return 0, io.EOF }
func (s *stubFile) ReadAt(_ []byte, _ int64) (int, error) { return 0, io.EOF }
func (s *stubFile) Seek(_ int64, _ int) (int64, error)    { return 0, nil }
func (s *stubFile) Close() error                          { return s.closeErr }

func TestSafeClose(t *testing.T) {
	t.Run("closes successfully", func(t *testing.T) {
		SafeClose(&stubFile{closeErr: nil})
	})

	t.Run("does not panic on close error", func(t *testing.T) {
		SafeClose(&stubFile{closeErr: errors.New("disk error")})
	})
}
