package code

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"syscall"
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
	cfg := project(filepath.Join(dir, "repo"))
	if err := Open(cfg, filepath.Join(dir, "cache")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Close)
	if err := Reindex(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	waitText(t, "NewOrder")
	return Roots()[0].ID
}

// project is a config with one active project of the given roots.
func project(paths ...string) *config.Config {
	p := config.Project{Name: "test"}
	for _, path := range paths {
		p.Roots = append(p.Roots, config.Root{Path: path})
	}
	return &config.Config{Active: p.Name, Projects: []config.Project{p}}
}

// waitText waits until text search finds q: searchers pick up new shards
// asynchronously.
func waitText(t *testing.T, q string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		hits, err := SearchText(context.Background(), q, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("text index did not load %q", q)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		abs := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProjects(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	work, home, side := filepath.Join(dir, "work"), filepath.Join(dir, "home"), filepath.Join(dir, "side")
	writeFiles(t, work, map[string]string{"billing/invoice.go": "package billing\n\ntype Invoice struct{}\n", "auth/token.go": "package auth\n\ntype Token struct{}\n"})
	writeFiles(t, home, map[string]string{"pet.go": "package pet\n\ntype Pet struct{}\n"})
	writeFiles(t, side, map[string]string{"side.go": "package side\n\ntype Side struct{}\n"})
	cfg := &config.Config{Active: "Work", Projects: []config.Project{
		// Two roots, one of them limited to a subdirectory.
		{Name: "Work", Roots: []config.Root{{Path: work, Include: []string{"billing"}}, {Path: side}}},
		{Name: "Home", Roots: []config.Root{{Path: home}}},
	}}
	open := func() {
		t.Helper()
		if err := Open(cfg, cache); err != nil {
			t.Fatal(err)
		}
		if err := Reindex(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
	}
	open()
	t.Cleanup(Close)
	waitText(t, "Invoice")
	waitText(t, "Side")
	if got := titles(SearchTypes("Pet", 10)); len(got) != 0 {
		t.Errorf("Work sees Home: %v", got)
	}
	if got := titles(SearchTypes("Token", 10)); len(got) != 0 {
		t.Errorf("Work sees a subdirectory it does not include: %v", got)
	}
	if hits, _ := SearchText(context.Background(), "package", 10); len(hits) != 2 {
		t.Errorf("SearchText(package) in Work = %v, want both roots", titles(hits))
	}
	workKeys := []string{cacheKey(cfg.Projects[0].Roots[0]), cacheKey(cfg.Projects[0].Roots[1])}
	workID := Roots()[0].ID

	cfg.Active = "Home"
	open()
	waitText(t, "Pet")
	if got := titles(SearchTypes("Invoice", 10)); len(got) != 0 {
		t.Errorf("Home sees Work: %v", got)
	}
	if hits, _ := SearchText(context.Background(), "Invoice", 10); len(hits) != 0 {
		t.Errorf("Home text search sees Work: %v", titles(hits))
	}
	for _, key := range workKeys {
		if !hasShards(key) {
			t.Errorf("Work cache %s was dropped on switching", key)
		}
	}

	// Changing a root's subdirectories keeps its ID, so open files still
	// resolve, but gives it a new cache; the old one goes.
	cfg.Active = "Work"
	cfg.Projects[0].Roots[0].Include = []string{"auth"}
	open()
	waitText(t, "Token")
	if got := Roots()[0].ID; got != workID {
		t.Errorf("root ID changed with its subdirectories: %s → %s", workID, got)
	}
	if _, err := ReadFile(workID, "auth/token.go"); err != nil {
		t.Errorf("ReadFile after changing subdirectories: %v", err)
	}
	if hasShards(workKeys[0]) {
		t.Errorf("stale cache %s kept", workKeys[0])
	}
	if !hasShards(workKeys[1]) {
		t.Errorf("cache %s of an unchanged root dropped", workKeys[1])
	}
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

func TestReindexListsFilesFirst(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, filepath.Join(dir, "repo"), fixture)
	if err := Open(project(filepath.Join(dir, "repo")), filepath.Join(dir, "cache")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Close)
	id := Roots()[0].ID
	listed := false
	err := Reindex(context.Background(), func(p Progress) {
		if !p.Listed {
			return
		}
		listed = true
		// The tree and file search work before symbols are extracted.
		if entries, err := ListDir(id, ""); err != nil || len(entries) == 0 {
			t.Errorf("ListDir while indexing = %v, %v", entries, err)
		}
		if got := SearchFiles("order", 10); len(got) != 3 {
			t.Errorf("SearchFiles while indexing = %v", titles(got))
		}
		if got := SearchTypes("Order", 10); len(got) != 0 {
			t.Errorf("symbols before extraction: %v", titles(got))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !listed {
		t.Error("no Listed progress for a root indexed for the first time")
	}
	if got := SearchTypes("Order", 10); len(got) == 0 {
		t.Error("no symbols after indexing")
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

func TestReindexRemovesUnfinishedShards(t *testing.T) {
	setup(t)
	r := roots[0]
	leftover := filepath.Join(shardDir(r.key), r.key+"_v16.00000.zoekt.123.tmp")
	if err := os.WriteFile(leftover, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.cfg.Path, "added.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Reindex(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(leftover); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("unfinished shard survived a rebuild: %v", err)
	}
	if !hasShards(r.key) {
		t.Error("rebuild left no shards")
	}
}

func TestOutOfResources(t *testing.T) {
	for _, c := range []struct {
		err  error
		want bool
	}{
		{&fs.PathError{Op: "open", Path: "a", Err: syscall.EMFILE}, true},
		{&fs.PathError{Op: "open", Path: "a", Err: syscall.ENFILE}, true},
		{&fs.PathError{Op: "open", Path: "a", Err: syscall.EACCES}, false},
		{&fs.PathError{Op: "open", Path: "a", Err: fs.ErrNotExist}, false},
	} {
		if got := outOfResources(c.err); got != c.want {
			t.Errorf("outOfResources(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

func TestListDir(t *testing.T) {
	id := setup(t)
	for _, c := range []struct {
		dir  string
		want []Entry
	}{
		{"", []Entry{{"api", true}, {"shop", true}, {"README.md", false}}},
		{"shop", []Entry{{"order.go", false}, {"Order.java", false}}},
		{"/shop/", []Entry{{"order.go", false}, {"Order.java", false}}},
	} {
		got, err := ListDir(id, c.dir)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("ListDir(%q) = %v, want %v", c.dir, got, c.want)
		}
	}
	if _, err := ListDir(id, "missing"); err == nil {
		t.Error("ListDir(missing): want error")
	}
}
