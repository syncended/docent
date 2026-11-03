package code

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/syncended/docent/core/config"
)

var fixture = map[string]string{
	"shop/order.go": `package shop

type OrderStatus int

const StatusDone OrderStatus = 1

func NewOrder() *Order { return &Order{} }
`,
	"shop/Order.java": `package shop;

public class Order {
    private String customerId;
    public String getCustomerId() { return customerId; }
}
`,
	"api/order.proto": `syntax = "proto3";
message OrderRequest { string order_id = 1; }
service OrderService { rpc Create(OrderRequest) returns (OrderRequest); }
`,
	"README.md": "Orders are created by NewOrder.\n",
}

// setup indexes a temporary root and waits until text search sees it.
func setup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for p, content := range fixture {
		abs := filepath.Join(dir, "repo", p)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{Roots: []config.Root{{Path: filepath.Join(dir, "repo")}}}
	if err := Open(cfg, filepath.Join(dir, "cache")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Close)
	if err := Reindex(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	// The searcher picks up new shards asynchronously.
	deadline := time.Now().Add(5 * time.Second)
	for {
		hits, err := SearchText(context.Background(), "NewOrder", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("text index did not load")
		}
		time.Sleep(20 * time.Millisecond)
	}
	return Roots()[0].ID
}

func titles(hits []Hit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, h.Title)
	}
	return out
}

func TestSearch(t *testing.T) {
	root := setup(t)
	ctx := context.Background()

	if got := titles(SearchTypes("OrdSt", 10)); !slices.Equal(got, []string{"OrderStatus"}) {
		t.Errorf("SearchTypes(OrdSt) = %v", got)
	}
	types := titles(SearchTypes("Order", 10))
	for _, want := range []string{"Order", "OrderStatus", "OrderRequest", "OrderService"} {
		if !slices.Contains(types, want) {
			t.Errorf("SearchTypes(Order) = %v, missing %s", types, want)
		}
	}
	if types[0] != "Order" {
		t.Errorf("exact match should rank first, got %v", types)
	}
	if got := SearchSymbols("Order.custId", 10); len(got) == 0 || got[0].Title != "customerId" || got[0].Container != "Order" {
		t.Errorf("SearchSymbols(Order.custId) = %+v", got)
	}

	files := SearchFiles("order", 10)
	if len(files) != 3 {
		t.Errorf("SearchFiles(order) = %v", titles(files))
	}
	if got := SearchFiles("api/ord:2", 10); len(got) != 1 || got[0].Path != "api/order.proto" || got[0].Line != 2 {
		t.Errorf("SearchFiles(api/ord:2) = %+v", got)
	}

	text, err := SearchText(ctx, "neworder", 10)
	if err != nil || len(text) != 2 {
		t.Errorf("SearchText(neworder) = %v, %v", titles(text), err)
	}
	if text, _ := SearchText(ctx, "NEWORDER", 10); len(text) != 2 {
		t.Errorf("SearchText should ignore case, got %v", titles(text))
	}
	if text, _ := SearchTextMatchCase(ctx, "Neworder", 10); len(text) != 0 {
		t.Errorf("SearchTextMatchCase(Neworder) = %v", titles(text))
	}
	if text, _ := SearchRegexMatchCase(ctx, `RPC \w+`, 10); len(text) != 0 {
		t.Errorf("SearchRegexMatchCase(RPC) = %v", titles(text))
	}
	regex, err := SearchRegex(ctx, `rpc \w+\(`, 10)
	if err != nil || len(regex) != 1 || regex[0].Line != 3 {
		t.Errorf("SearchRegex = %+v, %v", regex, err)
	}

	if defs := Definitions("Order", 10); len(defs) != 1 || defs[0].Path != "shop/Order.java" {
		t.Errorf("Definitions(Order) = %+v", defs)
	}
	usages, err := Usages(ctx, "OrderStatus", 10)
	if err != nil || len(usages) != 2 {
		t.Errorf("Usages(OrderStatus) = %+v, %v", usages, err)
	}

	sections := SearchAll(ctx, "NewOrder", 5)
	var tabs []Tab
	for _, s := range sections {
		tabs = append(tabs, s.Tab)
	}
	if !slices.Equal(tabs, []Tab{TabSymbols, TabText}) {
		t.Errorf("SearchAll(NewOrder) tabs = %v", tabs)
	}

	if content, err := ReadFile(root, "shop/order.go"); err != nil || content != fixture["shop/order.go"] {
		t.Errorf("ReadFile = %q, %v", content, err)
	}
	if _, err := ReadFile(root, "../../etc/passwd"); err == nil {
		t.Error("ReadFile outside the root should fail")
	}
}

func TestReindexKeepsUnchanged(t *testing.T) {
	setup(t)
	before := Roots()[0]
	if err := Reindex(context.Background(), func(Progress) { t.Error("unchanged root should not be rebuilt") }); err != nil {
		t.Fatal(err)
	}
	after := Roots()[0]
	if after.Files != before.Files || after.Symbols != before.Symbols {
		t.Errorf("reindex changed counts: %+v → %+v", before, after)
	}
}
