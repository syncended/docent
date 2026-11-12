package main

import (
	"errors"
	"io"
	"testing"
	"time"
)

// stutter returns its parts one per read, reporting the end of the body
// between them, as Wails does on macOS while WebKit is still delivering it.
type stutter struct{ parts []string }

func (s *stutter) Read(p []byte) (int, error) {
	if len(s.parts) == 0 {
		return 0, io.EOF
	}
	part := s.parts[0]
	s.parts = s.parts[1:]
	if part == "" {
		return 0, io.EOF
	}
	return copy(p, part), nil
}

func (s *stutter) Close() error { return nil }

func TestFullBodyWaitsThroughEarlyEnds(t *testing.T) {
	b := &fullBody{ReadCloser: &stutter{parts: []string{"abc", "", "", "def", ""}}, left: 6, deadline: time.Now().Add(time.Second)}
	got, err := io.ReadAll(b)
	if err != nil || string(got) != "abcdef" {
		t.Fatalf("ReadAll = %q, %v; want %q", got, err, "abcdef")
	}
}

func TestFullBodyFailsWhenShort(t *testing.T) {
	b := &fullBody{ReadCloser: &stutter{parts: []string{"abc"}}, left: 6, deadline: time.Now().Add(20 * time.Millisecond)}
	if _, err := io.ReadAll(b); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadAll error = %v, want io.ErrUnexpectedEOF", err)
	}
}
