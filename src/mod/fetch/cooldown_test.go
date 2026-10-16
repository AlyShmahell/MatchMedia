package fetch

import "testing"

func TestCircuitFailMS(t *testing.T) {
	c := NewCircuit()
	if c.Cooling("x") {
		t.Fatal("cold")
	}
	c.Fail("x", 2, 50, 100)
	if c.Cooling("x") {
		t.Fatal("need two fails")
	}
	c.Fail("x", 2, 50, 100)
	if !c.Cooling("x") {
		t.Fatal("want cooldown")
	}
	c.OK("x")
	if c.Cooling("x") {
		t.Fatal("cleared")
	}
}
