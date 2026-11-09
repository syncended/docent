// Command gen regenerates the bundled model catalog (catalog.json) from
// models.dev, keeping only the providers Docent has presets for and the
// models that can call tools. models.dev is MIT-licensed; its notice is in
// catalog.LICENSE.
//
// Run from core/provider: go generate, or go run ./gen [-src file-or-url].
package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/syncended/docent/core/provider"
)

type source map[string]struct {
	Models map[string]struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		Reasoning        bool   `json:"reasoning"`
		ToolCall         bool   `json:"tool_call"`
		Status           string `json:"status"`
		ReleaseDate      string `json:"release_date"`
		ReasoningOptions []struct {
			Type   string   `json:"type"`
			Values []string `json:"values"`
		} `json:"reasoning_options"`
		Limit struct {
			Context int `json:"context"`
			Output  int `json:"output"`
		} `json:"limit"`
	} `json:"models"`
}

func main() {
	src := flag.String("src", "https://models.dev/api.json", "models.dev api.json, a URL or a file")
	out := flag.String("out", "catalog.json", "output file")
	flag.Parse()

	data, err := read(*src)
	if err != nil {
		log.Fatal(err)
	}
	var in source
	if err := json.Unmarshal(data, &in); err != nil {
		log.Fatal(err)
	}

	catalog := map[string][]provider.Model{}
	for _, p := range provider.All() {
		for _, e := range p.Endpoints {
			for _, c := range e.Catalogs {
				entry, ok := in[c]
				if !ok {
					log.Fatalf("models.dev has no provider %q (%s)", c, p.ID)
				}
				models := []provider.Model{}
				for _, m := range entry.Models {
					if !m.ToolCall || m.Status == "deprecated" {
						continue
					}
					model := provider.Model{
						ID: m.ID, Name: m.Name, Context: m.Limit.Context, Output: m.Limit.Output,
						Reasoning: m.Reasoning, Released: m.ReleaseDate,
					}
					for _, o := range m.ReasoningOptions {
						if o.Type == "effort" {
							model.Efforts = o.Values
						}
					}
					models = append(models, model)
				}
				slices.SortFunc(models, func(a, b provider.Model) int {
					return cmp.Or(strings.Compare(b.Released, a.Released), strings.Compare(a.ID, b.ID))
				})
				catalog[c] = models
			}
		}
	}
	for _, p := range provider.All() {
		if p.Recommended == "" {
			continue
		}
		if !inCatalog(catalog, p, p.Recommended) {
			log.Printf("warning: %s recommends %s, which models.dev does not list", p.ID, p.Recommended)
		}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", " ")
	if err := enc.Encode(catalog); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	n := 0
	for _, ms := range catalog {
		n += len(ms)
	}
	fmt.Printf("%s: %d models from %d providers\n", *out, n, len(catalog))
}

func inCatalog(catalog map[string][]provider.Model, p provider.Provider, id string) bool {
	for _, c := range p.Endpoints[0].Catalogs {
		for _, m := range catalog[c] {
			if m.ID == id {
				return true
			}
		}
	}
	return false
}

func read(src string) ([]byte, error) {
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		return os.ReadFile(src)
	}
	res, err := http.Get(src)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", src, res.Status)
	}
	return io.ReadAll(res.Body)
}
