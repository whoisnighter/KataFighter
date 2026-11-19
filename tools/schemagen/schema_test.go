package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

const ikemenSrc = "../../ikemen/source/src"

func generate(t *testing.T) *Schema {
	t.Helper()
	if _, err := os.Stat(ikemenSrc); err != nil {
		t.Skip("ikemen/source submodule not checked out")
	}
	s, err := Generate(ikemenSrc)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func find[T any](list []T, name func(T) string, n string) *T {
	for i := range list {
		if name(list[i]) == n {
			return &list[i]
		}
	}
	return nil
}

func TestKnownControllersAndTriggers(t *testing.T) {
	s := generate(t)
	ctrl := func(n string) *Controller {
		c := find(s.Controllers, func(c Controller) string { return c.Name }, n)
		if c == nil {
			t.Fatalf("controller %s missing", n)
		}
		return c
	}
	param := func(c *Controller, n string) *Param {
		p := find(c.Params, func(p Param) string { return p.Name }, n)
		if p == nil {
			t.Fatalf("%s: param %s missing", c.Name, n)
		}
		return p
	}

	if p := param(ctrl("changestate"), "value"); !p.Mandatory || p.Type != "Int" {
		t.Errorf("changestate.value = %+v", *p)
	}
	if p := param(ctrl("hitdef"), "damage"); p.Type != "Int" || p.Count != 2 {
		t.Errorf("hitdef.damage = %+v", *p)
	}
	param(ctrl("explod"), "trans") // via paramTrans helper with prefix ""
	param(ctrl("explod"), "alpha")
	if c := ctrl("velset"); c.Origin != "mugen" {
		t.Errorf("velset origin = %s", c.Origin)
	}
	if c := ctrl("modifyexplod"); c.Origin != "mugen" && c.Origin != "ikemen" {
		t.Errorf("modifyexplod origin = %q", c.Origin)
	}

	trig := func(n string) *Trigger {
		tr := find(s.Triggers, func(t Trigger) string { return t.Name }, n)
		if tr == nil {
			t.Fatalf("trigger %s missing", n)
		}
		return tr
	}
	found := false
	for _, k := range trig("gethitvar").Keywords {
		found = found || k == "damage"
	}
	if !found {
		t.Error("gethitvar(damage) keyword missing")
	}
	if tr := trig("parent"); tr.Origin != "redirection" {
		t.Errorf("parent origin = %s", tr.Origin)
	}
	for _, tr := range s.Triggers {
		if !keywordRe.MatchString(tr.Name) {
			t.Errorf("operator leaked into triggers: %q", tr.Name)
		}
	}
}

// The committed lsp/schema/ikemen.json must match a fresh run. If this fails,
// regenerate: go run ./schemagen (from tools/).
func TestCommittedSchemaUpToDate(t *testing.T) {
	s := generate(t)
	b, err := os.ReadFile("../../lsp/schema/ikemen.json")
	if err != nil {
		t.Fatal(err)
	}
	var committed Schema
	if err := json.Unmarshal(b, &committed); err != nil {
		t.Fatal(err)
	}
	// Round-trip the fresh schema through JSON so empty/nil slices compare equal
	fb, _ := json.Marshal(s)
	var fresh Schema
	json.Unmarshal(fb, &fresh)
	if !reflect.DeepEqual(committed.Controllers, fresh.Controllers) ||
		!reflect.DeepEqual(committed.Triggers, fresh.Triggers) ||
		!reflect.DeepEqual(committed.Warnings, fresh.Warnings) {
		t.Error("lsp/schema/ikemen.json is stale; run `go run ./schemagen` in tools/")
	}
}
