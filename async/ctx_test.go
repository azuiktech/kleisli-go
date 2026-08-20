package async_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/azuiktech/kleisli-go/async"
)

func TestInCtx_stores_context_and_value(t *testing.T) {
	ctx := context.WithValue(context.Background(), "k", "v")
	c := async.InCtx(ctx, 42)
	if c.Context != ctx {
		t.Fatalf("wrong context stored")
	}
	if c.Val != 42 {
		t.Fatalf("want 42, got %d", c.Val)
	}
}

func TestCtx_literal_construction(t *testing.T) {
	ctx := context.Background()
	c := async.Ctx[string]{Context: ctx, Val: "hello"}
	if c.Val != "hello" {
		t.Fatalf("want hello, got %s", c.Val)
	}
}

func TestCtx_with_http_client(t *testing.T) {
	ctx := context.Background()
	client := &http.Client{}
	c := async.InCtx(ctx, client)
	if c.Val != client {
		t.Fatalf("wrong client stored")
	}
	if c.Context != ctx {
		t.Fatalf("wrong context stored")
	}
}
