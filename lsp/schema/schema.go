// Package schema loads ikemen.json, the list of state controllers and
// triggers that tools/schemagen generates from the pinned Ikemen GO source.
// The JSON is embedded, so the server needs no data files at run time.
package schema

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed ikemen.json
var ikemenJSON []byte

// Param is one controller parameter.
type Param struct {
	Name string `json:"name"`
	// Type is "Int", "Float", "Bool" or "" when the value is not a plain
	// expression (alpha, trans, attr, ...).
	Type      string `json:"type,omitempty"`
	Count     int    `json:"count,omitempty"`
	Mandatory bool   `json:"mandatory,omitempty"`
}

// Controller is a state controller type.
type Controller struct {
	Name   string  `json:"name"`
	Origin string  `json:"origin"` // "mugen" or "ikemen"
	Source string  `json:"source"`
	Params []Param `json:"params"`
	// Unresolved lists parameter names the generator could not read because
	// they are built at run time (varset var(N), mapset keys, helper size
	// keys). A controller with any is open-ended: unknown keys are allowed.
	Unresolved []string `json:"unresolved,omitempty"`
}

// Param returns the parameter with the given lower-case name, or nil.
func (c *Controller) Param(name string) *Param {
	for i := range c.Params {
		if c.Params[i].Name == name {
			return &c.Params[i]
		}
	}
	return nil
}

// Trigger is a trigger or function usable in expressions.
type Trigger struct {
	Name string `json:"name"`
	// Origin is "mugen", "ikemen", "redirection" or "other" (accepted by
	// the compiler but missing from its trigger map).
	Origin   string   `json:"origin"`
	Keywords []string `json:"keywords,omitempty"`
}

// HasKeyword reports whether k (lower-case) is one of the trigger's keyword
// arguments, as in const(size.xscale) or gethitvar(damage).
func (t *Trigger) HasKeyword(k string) bool {
	for _, w := range t.Keywords {
		if w == k {
			return true
		}
	}
	return false
}

// Schema is the decoded ikemen.json.
type Schema struct {
	Source struct {
		Engine  string `json:"engine"`
		Version string `json:"version"`
		Commit  string `json:"commit"`
	} `json:"source"`
	// CNSKeywords are the controller keys that are not parameters in CNS
	// (type, persistent, ignorehitpause, triggerall, trigger<N>).
	CNSKeywords []string     `json:"cnsKeywords"`
	Controllers []Controller `json:"controllers"`
	Triggers    []Trigger    `json:"triggers"`
	Warnings    []string     `json:"warnings"`

	controllers map[string]*Controller
	triggers    map[string]*Trigger
}

// Parse decodes a schema.
func Parse(b []byte) (*Schema, error) {
	s := &Schema{}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	s.controllers = make(map[string]*Controller, len(s.Controllers))
	for i := range s.Controllers {
		s.controllers[s.Controllers[i].Name] = &s.Controllers[i]
	}
	s.triggers = make(map[string]*Trigger, len(s.Triggers))
	for i := range s.Triggers {
		s.triggers[s.Triggers[i].Name] = &s.Triggers[i]
	}
	return s, nil
}

var (
	defaultOnce   sync.Once
	defaultSchema *Schema
)

// Default returns the embedded schema. It panics if the embedded JSON is
// invalid, which the tests rule out.
func Default() *Schema {
	defaultOnce.Do(func() {
		s, err := Parse(ikemenJSON)
		if err != nil {
			panic("lsp/schema: embedded ikemen.json: " + err.Error())
		}
		defaultSchema = s
	})
	return defaultSchema
}

// Controller returns the controller type with the given name (any case),
// or nil.
func (s *Schema) Controller(name string) *Controller {
	return s.controllers[strings.ToLower(name)]
}

// Trigger returns the trigger with the given name (any case), or nil.
// The legacy projectile forms with an ID suffix (projhit1200,
// projguarded12, projcontact3) resolve to their base trigger, as in
// Ikemen's expValue.
func (s *Schema) Trigger(name string) *Trigger {
	name = strings.ToLower(name)
	if t := s.triggers[name]; t != nil {
		return t
	}
	for _, base := range [...]string{"projhit", "projguarded", "projcontact"} {
		if strings.HasPrefix(name, base) {
			return s.triggers[base]
		}
	}
	return nil
}

// IsCNSKeyword reports whether key (lower-case) is a CNS controller keyword
// rather than a parameter.
func (s *Schema) IsCNSKeyword(key string) bool {
	for _, k := range s.CNSKeywords {
		if k == key {
			return true
		}
	}
	return false
}
