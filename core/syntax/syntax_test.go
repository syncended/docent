package syntax

import (
	"context"
	"testing"
	"unicode/utf16"
)

func classes(t *testing.T, path, src string) map[string]string {
	t.Helper()
	spans, err := Highlight(context.Background(), path, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	units := utf16.Encode([]rune(src))
	// Later (inner) spans override earlier ones, as when rendered.
	out := map[string]string{}
	for _, s := range spans {
		out[string(utf16.Decode(units[s.From:s.To]))] = s.Class
	}
	return out
}

func TestKotlin(t *testing.T) {
	src := `package x.y
import kotlin.math.max
@Suppress("unused")
data class User(val id: Long, private var name: String = "имя ${id}") : Base() {
    companion object { const val MAX = 10L; val ok = true; val n: String? = null }
    override suspend fun load(limit: Int): List<User> = repo.find(limit)
}
enum class Color { RED }
// note
`
	got := classes(t, "User.kt", src)
	for text, class := range map[string]string{
		"class": "keyword", "data": "keyword", "override": "keyword", "suspend": "keyword", "fun": "keyword",
		"User": "type", "String": "type", "load": "function", "find": "function", "id": "variable",
		"name": "property", "limit": "parameter", "10L": "number", "true": "constant", "null": "constant",
		"RED": "constant", "// note": "comment", `"unused"`: "string", "x.y": "namespace",
	} {
		if got[text] != class {
			t.Errorf("%q: got %q, want %q", text, got[text], class)
		}
	}
}

func TestProto(t *testing.T) {
	src := `syntax = "proto3";
package shop.v1;
// note
message Order {
  string id = 1;
  map<string, int32> tags = 2;
  enum Kind { KIND_UNKNOWN = 0; }
}
service Shop { rpc Buy(Order) returns (stream Order); }
`
	got := classes(t, "shop.proto", src)
	for text, class := range map[string]string{
		"message": "keyword", "service": "keyword", "returns": "keyword", "Order": "type", "Shop": "type",
		"string": "type", "id": "property", "tags": "property", "KIND_UNKNOWN": "constant", "Buy": "function",
		"1": "number", "// note": "comment", `"proto3"`: "string", "shop.v1": "namespace",
	} {
		if got[text] != class {
			t.Errorf("%q: got %q, want %q", text, got[text], class)
		}
	}
}

func TestUTF16Offsets(t *testing.T) {
	// "я" is 2 bytes and 1 UTF-16 unit; "😀" is 4 bytes and 2 units.
	conv := utf16Offsets([]byte("aя😀b"))
	for b, want := range map[uint32]int{0: 0, 1: 1, 3: 2, 7: 4, 8: 5} {
		if got := conv(b); got != want {
			t.Errorf("byte %d: got %d, want %d", b, got, want)
		}
	}
}
