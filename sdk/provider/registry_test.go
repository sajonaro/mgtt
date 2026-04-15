package provider

import (
	"context"
	"errors"
	"testing"
)

func TestRegistry_UnknownType(t *testing.T) {
	r := NewRegistry()
	_, err := r.Probe(context.Background(), Request{Type: "bogus", Fact: "x"})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("expected ErrUsage, got %v", err)
	}
}

func TestRegistry_UnknownFact(t *testing.T) {
	r := NewRegistry()
	r.Register("foo", map[string]ProbeFn{
		"known": func(ctx context.Context, req Request) (Result, error) { return IntResult(1), nil },
	})
	_, err := r.Probe(context.Background(), Request{Type: "foo", Fact: "missing"})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("expected ErrUsage, got %v", err)
	}
}

func TestRegistry_DispatchAndStatusDefault(t *testing.T) {
	r := NewRegistry()
	r.Register("foo", map[string]ProbeFn{
		"bar": func(ctx context.Context, req Request) (Result, error) {
			return Result{Value: 42, Raw: "42"}, nil
		},
	})
	got, err := r.Probe(context.Background(), Request{Type: "foo", Fact: "bar"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != 42 {
		t.Fatalf("expected 42, got %v", got.Value)
	}
	if got.Status != StatusOk {
		t.Fatalf("Status should default to ok, got %q", got.Status)
	}
}

func TestRegistry_NotFoundErrTranslatesToStatus(t *testing.T) {
	r := NewRegistry()
	r.Register("foo", map[string]ProbeFn{
		"bar": func(ctx context.Context, req Request) (Result, error) {
			return Result{}, ErrNotFound
		},
	})
	got, err := r.Probe(context.Background(), Request{Type: "foo", Fact: "bar"})
	if err != nil {
		t.Fatalf("not_found should not surface as error: %v", err)
	}
	if got.Status != StatusNotFound {
		t.Fatalf("want not_found, got %q", got.Status)
	}
	if got.Value != nil {
		t.Fatalf("Value should be nil on not_found, got %v", got.Value)
	}
}
