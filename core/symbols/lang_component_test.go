package symbols

import (
	"context"
	"testing"
)

func TestExtractSvelte(t *testing.T) {
	checkExtract(t, "svelte", `<script lang="ts">
  export let title: string;
  function close() { const local = 1; }
</script>

<h1>{title}</h1>
`, []string{"title variable", "close function"}, []string{"local"})
}

func TestExtractVue(t *testing.T) {
	src := `<template>
  <div>{{ count }}</div>
</template>

<script setup lang="ts">
import { ref } from "vue";
interface Props { label: string }
const count = ref(0);
function increment() { count.value++; }
</script>
`
	checkExtract(t, "vue", src, []string{"Props interface", "Props.label field", "count variable", "increment function"}, nil)
	syms, err := Extract(context.Background(), "vue", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range syms {
		if src[s.Start:s.End] != s.Name {
			t.Errorf("%s at %d:%d covers %q", s.Name, s.Start, s.End, src[s.Start:s.End])
		}
		if s.Name == "Props" && (s.Line != 7 || s.Column != 10) {
			t.Errorf("Props at line %d col %d, want 7:10", s.Line, s.Column)
		}
	}
}
