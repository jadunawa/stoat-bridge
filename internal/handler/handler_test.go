package handler

import (
	"net/http"
	"testing"

	"github.com/jadunawa/stoat-bridge/internal/message"
)

type stubHandler struct {
	name string
}

func (s *stubHandler) Name() string { return s.name }
func (s *stubHandler) Parse(_ *http.Request) ([]message.Message, error) {
	return nil, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	h := &stubHandler{name: "test"}
	reg.Register(h)

	got, ok := reg.Get("test")
	if !ok {
		t.Fatal("expected handler to be found")
	}
	if got.Name() != "test" {
		t.Errorf("name = %q, want %q", got.Name(), "test")
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	reg := NewRegistry()
	_, ok := reg.Get("nonexistent")
	if ok {
		t.Fatal("expected handler to not be found")
	}
}

func TestRegistry_Handlers(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubHandler{name: "a"})
	reg.Register(&stubHandler{name: "b"})

	handlers := reg.Handlers()
	if len(handlers) != 2 {
		t.Errorf("got %d handlers, want 2", len(handlers))
	}
	if _, ok := handlers["a"]; !ok {
		t.Error("missing handler 'a'")
	}
	if _, ok := handlers["b"]; !ok {
		t.Error("missing handler 'b'")
	}
}
