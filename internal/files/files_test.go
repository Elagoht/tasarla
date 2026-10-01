package files_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"kanban/internal/files"
)

func TestSaveDetectsTheTypeAndStoresUnderARandomKey(t *testing.T) {
	dir, err := files.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 100)...)
	saved, err := dir.Save(bytes.NewReader(png), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ContentType != "image/png" || saved.Size != int64(len(png)) || len(saved.Key) < 32 || strings.ContainsAny(saved.Key, "/.") {
		t.Fatalf("saved = %+v", saved)
	}
	f, err := dir.Open(saved.Key)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(f)
	f.Close()
	if !bytes.Equal(got, png) {
		t.Fatal("content differs")
	}
	if err := dir.Remove(saved.Key); err != nil {
		t.Fatal(err)
	}
	if _, err := dir.Open(saved.Key); err == nil {
		t.Fatal("a removed file opened")
	}
}

func TestSaveRefusesWhatIsTooLarge(t *testing.T) {
	dir, _ := files.Open(t.TempDir())
	_, err := dir.Save(strings.NewReader(strings.Repeat("x", 11)), 10)
	if !errors.Is(err, files.ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestOpenRefusesKeysThatLeaveTheDirectory(t *testing.T) {
	dir, _ := files.Open(t.TempDir())
	for _, key := range []string{"../etc/passwd", "/etc/passwd", "a/b", ""} {
		if _, err := dir.Open(key); err == nil {
			t.Errorf("Open(%q) succeeded", key)
		}
	}
}
