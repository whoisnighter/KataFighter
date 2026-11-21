package schema

import "testing"

func TestDefault(t *testing.T) {
	s := Default()
	if s.Source.Engine != "Ikemen GO" || s.Source.Commit == "" {
		t.Errorf("source = %+v", s.Source)
	}
	if len(s.Controllers) < 100 || len(s.Triggers) < 200 {
		t.Fatalf("%d controllers, %d triggers", len(s.Controllers), len(s.Triggers))
	}
	c := s.Controller("ChangeState")
	if c == nil {
		t.Fatal("no changestate")
	}
	if p := c.Param("value"); p == nil || p.Type != "Int" || !p.Mandatory {
		t.Errorf("changestate value = %+v", p)
	}
	if c.Param("nope") != nil {
		t.Error("unexpected param")
	}
	if s.Controller("nope") != nil {
		t.Error("unexpected controller")
	}
}

func TestTrigger(t *testing.T) {
	s := Default()
	for _, name := range []string{"Time", "animelemtime", "parent", "ProjHit1200", "projguarded", "projcontact3"} {
		if s.Trigger(name) == nil {
			t.Errorf("Trigger(%q) = nil", name)
		}
	}
	if s.Trigger("tme") != nil {
		t.Error("tme should be unknown")
	}
	if c := s.Trigger("const"); c == nil || !c.HasKeyword("size.xscale") {
		t.Error("const should have size.xscale")
	}
	if !s.IsCNSKeyword("type") || s.IsCNSKeyword("value") {
		t.Error("IsCNSKeyword")
	}
}
