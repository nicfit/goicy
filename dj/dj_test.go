package dj

import "testing"

var (
	nicfit  = Dj{Name: "nicfit"}
	djbotch = Dj{Name: "DjBotch"}
	nickw   = Dj{Name: "Nick Warren"}
	n0pe    = Dj{Name: "N0pe"}
)

func TestBooth_JoinLeave(t *testing.T) {
	booth := NewBooth()
	if booth.Size() != 0 {
		t.Fail()
	}
	booth.Join(nicfit)
	if booth.Size() != 1 {
		t.Fail()
	}
	booth.Join(djbotch)
	if booth.Size() != 2 {
		t.Fail()
	}
	booth.Join(nicfit)
	if booth.Size() != 2 {
		t.Fail()
	}

	booth.Leave(Dj{Name: "Nick"})
	if booth.Size() != 2 {
		t.Fail()
	}
	booth.Leave(djbotch)
	if booth.Size() != 1 {
		t.Fail()
	}
	booth.Leave(nicfit)
	if booth.Size() != 0 {
		t.Fail()
	}
}

func TestBooth_Cycle(t *testing.T) {
	booth := NewBooth()
	if _, err := booth.Cycle(); err == nil {
		t.Fail()
	}

	booth.Join(djbotch)
	booth.Join(nicfit)
	booth.Join(n0pe)
	booth.Join(nickw)
	if booth.Size() != 4 {
		t.Fail()
	}
	if dj, err := booth.Cycle(); err != nil || dj != djbotch {
		t.Fail()
	}

	expected := []Dj{nicfit, n0pe, nickw, djbotch}
	for i := range 4 {
		if dj, err := booth.Cycle(); err != nil || dj != expected[i] {
			t.Fail()
		}
	}

	booth.Leave(n0pe)
	expected = []Dj{nicfit, nickw, djbotch, nicfit}
	for i := range 4 {
		if dj, err := booth.Cycle(); err != nil || dj != expected[i] {
			t.Fail()
		}
	}
}
