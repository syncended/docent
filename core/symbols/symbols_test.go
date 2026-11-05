package symbols

import (
	"context"
	"fmt"
	"slices"
	"testing"
)

func TestLanguage(t *testing.T) {
	for p, want := range map[string]string{
		"a/b.go": "go", "X.java": "java", "x.kt": "kotlin", "x.KTS": "kotlin",
		"x.hpp": "cpp", "x.cc": "cpp", "api.proto": "proto", "x.mjs": "javascript",
		"x.ts": "typescript", "x.tsx": "tsx", "README.md": "", "Makefile": "",
	} {
		if got := Language(p); got != want {
			t.Errorf("Language(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestExtract(t *testing.T) {
	tests := []struct {
		lang string
		src  string
		want []string // "parent.name kind"
		not  []string // names that must not be extracted
	}{
		{
			lang: "go",
			src: `package x
type Order struct { ID string; total int }
type Store interface { Get(id string) Order }
type Status int
const ( Unknown Status = iota; Done )
var defaultStore Store
func New() *Order { local := 1; _ = local; return nil }
func (o *Order) Total() int { return o.total }
`,
			want: []string{
				"Order struct", "Order.ID field", "Order.total field",
				"Store interface", "Store.Get method", "Status type",
				"Unknown constant", "Done constant", "defaultStore variable",
				"New function", "Total method",
			},
			not: []string{"local"},
		},
		{
			lang: "java",
			src: `package x;
public class Order {
  public static final int MAX = 1;
  private String id;
  public String getId() { int local = 0; return id; }
  enum Status { NEW, DONE }
}
interface Store { Order get(String id); }
record Point(int x, int y) {}
@interface Marker {}
`,
			want: []string{
				"Order class", "Order.MAX field", "Order.id field", "Order.getId method",
				"Order.Status enum", "Status.NEW enum_value", "Status.DONE enum_value",
				"Store interface", "Store.get method", "Point class", "Marker interface",
			},
			not: []string{"local"},
		},
		{
			lang: "kotlin",
			src: `package x
enum class Color { RED, GREEN }
data class User(val id: Long) {
  val age: Int = 1
  fun name(): String { val local = 1; return "" }
}
interface Repo { fun get(): User }
object Single { const val MAX = 1 }
typealias Id = Long
val topLevel = 1
fun top() {}
`,
			want: []string{
				"Color enum", "Color.RED enum_value", "Color.GREEN enum_value",
				"User class", "User.age field", "User.name method",
				"Repo interface", "Repo.get method", "Single object", "Single.MAX field",
				"Id type", "topLevel variable", "top function",
			},
			not: []string{"local"},
		},
		{
			lang: "cpp",
			src: `namespace shop {
enum class Status { kNew, kDone };
struct Point { int x; };
class Order {
 public:
  int Total() const;
 private:
  int total_;
};
using Id = long;
typedef int Count;
int Order::Total() const { int local = 0; return total_ + local; }
Order* Make();
}
`,
			want: []string{
				"shop namespace", "shop.Status enum", "Status.kNew enum_value",
				"Status.kDone enum_value", "shop.Point struct", "Point.x field",
				"shop.Order class", "Order.Total method", "Order.total_ field",
				"shop.Id type", "shop.Count type", "Order.Total method",
				"shop.Make function",
			},
			not: []string{"local"},
		},
		{
			lang: "proto",
			src: `syntax = "proto3";
message Order {
  string id = 1;
  map<string, int32> tags = 2;
  oneof payment { string card = 3; }
  enum Kind { KIND_UNKNOWN = 0; }
  message Item {}
}
enum Status { STATUS_UNKNOWN = 0; STATUS_DONE = 1; }
service Shop { rpc Buy(Order) returns (Order); }
`,
			want: []string{
				"Order message", "Order.id field", "Order.tags field", "Order.card field",
				"Order.Kind enum", "Kind.KIND_UNKNOWN enum_value", "Order.Item message",
				"Status enum", "Status.STATUS_DONE enum_value",
				"Shop service", "Shop.Buy rpc",
			},
		},
		{
			lang: "javascript",
			src: `export class Cart {
  items = [];
  total() { const local = 1; return local; }
}
function discount() {}
export const apply = (x) => x;
const LIMIT = 10;
`,
			want: []string{
				"Cart class", "Cart.items field", "Cart.total method",
				"discount function", "apply function", "LIMIT variable",
			},
			not: []string{"local"},
		},
		{
			lang: "typescript",
			src: `export interface Cart { items: string[]; total(): number; }
export enum Status { New, Done = 2 }
export type Id = string;
export abstract class Base { abstract run(): void; }
export class Impl extends Base { private count = 0; run() { const local = 1; } }
declare function helper(x: number): void;
namespace Utils { export const VERSION = 1; }
`,
			want: []string{
				"Cart interface", "Cart.items field", "Cart.total method",
				"Status enum", "Status.New enum_value", "Status.Done enum_value",
				"Id type", "Base class", "Base.run method",
				"Impl class", "Impl.count field", "Impl.run method",
				"helper function", "Utils namespace", "Utils.VERSION variable",
			},
			not: []string{"local"},
		},
		{
			lang: "tsx",
			src:  `export const Button = (props: Props) => <button>{props.label}</button>;`,
			want: []string{"Button function"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) { checkExtract(t, tt.lang, tt.src, tt.want, tt.not) })
	}
}

// checkExtract extracts the declarations of src, written in lang, and checks
// that each of want ("parent.name kind") is among them and none of the names
// in not is.
func checkExtract(t *testing.T, lang, src string, want, not []string) {
	t.Helper()
	syms, err := Extract(context.Background(), lang, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range syms {
		name := s.Name
		if s.Parent != "" {
			name = s.Parent + "." + name
		}
		got = append(got, fmt.Sprintf("%s %s", name, s.Kind))
		if slices.Contains(not, s.Name) {
			t.Errorf("unexpected symbol %s %s", name, s.Kind)
		}
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("missing %q", w)
		}
	}
	if t.Failed() {
		t.Logf("got: %q", got)
	}
}

func TestExtractPositions(t *testing.T) {
	src := "package x\n\ntype Order struct{}\n"
	syms, err := Extract(context.Background(), "go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 1 {
		t.Fatalf("got %d symbols, want 1", len(syms))
	}
	s := syms[0]
	if s.Line != 3 || s.Column != 5 || src[s.Start:s.End] != "Order" {
		t.Errorf("got line %d col %d text %q", s.Line, s.Column, src[s.Start:s.End])
	}
}
