package fuzzy

import (
	"slices"
	"sort"
	"testing"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		query, name string
		match       bool
		pos         []int
	}{
		{"OrderStatus", "OrderStatus", true, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
		{"order", "OrderStatus", true, []int{0, 1, 2, 3, 4}},
		{"OrSt", "OrderStatus", true, []int{0, 1, 5, 6}},
		{"os", "OrderStatus", true, []int{0, 5}},
		{"status", "OrderStatus", true, []int{5, 6, 7, 8, 9, 10}},
		{"hs", "HTTPServer", true, []int{0, 4}},
		{"user_id", "user_id", true, nil},
		{"uid", "user_id", true, []int{0, 5, 6}},
		{"rdr", "OrderStatus", false, nil},
		{"rder", "OrderStatus", true, []int{1, 2, 3, 4}},
		{"OrSt", "OrderRequest", false, nil},
		{"bid.proto", "bid_modifier_input.proto", true, []int{0, 1, 2, 18, 19, 20, 21, 22, 23}},
		{"bmi.proto", "bid_modifier_input.proto", true, []int{0, 4, 13, 18, 19, 20, 21, 22, 23}},
		{"xyz", "OrderStatus", false, nil},
		{"", "OrderStatus", false, nil},
	}
	for _, tt := range tests {
		s, pos := Match(tt.query, tt.name)
		if (s > 0) != tt.match {
			t.Errorf("Match(%q, %q) matched = %v, want %v", tt.query, tt.name, s > 0, tt.match)
			continue
		}
		if tt.pos != nil && !slices.Equal(pos, tt.pos) {
			t.Errorf("Match(%q, %q) pos = %v, want %v", tt.query, tt.name, pos, tt.pos)
		}
	}
}

func TestRanking(t *testing.T) {
	names := []string{
		"OrderStatusMapper", // humps from start, query case, longer
		"orderStatus",       // humps from start, different case
		"OrderStatus",       // humps from start, query case
		"MyOrderStatus",     // humps, not from start
		"BorderStatus",      // no match: "OrSt" is not a substring
	}
	sort.SliceStable(names, func(i, j int) bool {
		si, _ := Match("OrSt", names[i])
		sj, _ := Match("OrSt", names[j])
		return si > sj
	})
	want := []string{"OrderStatus", "OrderStatusMapper", "orderStatus", "MyOrderStatus", "BorderStatus"}
	if !slices.Equal(names, want) {
		t.Errorf("ranking = %v, want %v", names, want)
	}

	exact, _ := Match("Order", "Order")
	prefix, _ := Match("Order", "OrderStatus")
	if exact <= prefix {
		t.Errorf("exact %d should outrank prefix %d", exact, prefix)
	}
}
