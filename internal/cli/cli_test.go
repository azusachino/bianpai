package cli

import (
	"testing"

	"github.com/aisk/bianpai/internal/backend"
)

func TestNewBackendSelectsByName(t *testing.T) {
	if b, err := newBackend("container"); err != nil {
		t.Fatalf("container: %v", err)
	} else if _, ok := b.(*backend.Container); !ok {
		t.Fatalf("container -> %T, want *backend.Container", b)
	}

	if b, err := newBackend("wslc"); err != nil {
		t.Fatalf("wslc: %v", err)
	} else if _, ok := b.(*backend.WSLC); !ok {
		t.Fatalf("wslc -> %T, want *backend.WSLC", b)
	}

	if b, err := newBackend(""); err != nil {
		t.Fatalf("default: %v", err)
	} else if _, ok := b.(*backend.WSLC); !ok {
		t.Fatalf("default -> %T, want *backend.WSLC", b)
	}

	if _, err := newBackend("bogus"); err == nil {
		t.Fatal("expected error for unknown backend")
	}
}
