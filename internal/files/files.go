// Package files keeps attachments on disk (spec §9): one file per attachment,
// named by a random key, in one directory opened with os.OpenRoot so that no
// key can reach outside it.
package files

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
)

// ErrTooLarge refuses a file over the size given to Save.
var ErrTooLarge = errors.New("files: the file is too large")

var errBadKey = errors.New("files: malformed key")

var keyPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Dir is the attachments directory.
type Dir struct {
	root *os.Root
}

// Saved is what Save wrote.
type Saved struct {
	Key         string
	ContentType string
	Size        int64
}

// Open opens path, creating it if needed.
func Open(path string) (*Dir, error) {
	if err := os.MkdirAll(path, 0o750); err != nil {
		return nil, fmt.Errorf("files: %w", err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("files: %w", err)
	}
	return &Dir{root: root}, nil
}

// Save writes r under a new key. The content type comes from the first 512
// bytes, never from what the client claimed. More than maxSize bytes is
// ErrTooLarge, and nothing is kept.
func (d *Dir) Save(r io.Reader, maxSize int64) (Saved, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return Saved{}, err
	}
	key := hex.EncodeToString(raw)
	f, err := d.root.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return Saved{}, err
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		f.Close()
		d.root.Remove(key)
		return Saved{}, err
	}
	head = head[:n]
	written, err := io.Copy(f, io.LimitReader(io.MultiReader(bytes.NewReader(head), r), maxSize+1))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil && written > maxSize {
		err = ErrTooLarge
	}
	if err != nil {
		d.root.Remove(key)
		return Saved{}, err
	}
	return Saved{Key: key, ContentType: http.DetectContentType(head), Size: written}, nil
}

// Open opens the file saved under key.
func (d *Dir) Open(key string) (*os.File, error) {
	if !keyPattern.MatchString(key) {
		return nil, errBadKey
	}
	return d.root.Open(key)
}

// Remove deletes the file saved under key.
func (d *Dir) Remove(key string) error {
	if !keyPattern.MatchString(key) {
		return errBadKey
	}
	return d.root.Remove(key)
}
