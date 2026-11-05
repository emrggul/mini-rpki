package vrp

import (
	"path/filepath"
	"testing"
)

func TestSerialWrap(t *testing.T) {
	s, _ := Open("", nil)
	s.serial = ^uint32(0)
	s.history = []Snapshot{{s.serial, nil}}
	id, before, _ := s.State()
	s.Edit(rec("192.0.2.0/24", 24, 1), true)
	_, after, adds, _, ok := s.Delta(id, before)
	if !ok || after != 0 || len(adds) != 1 {
		t.Fatal(after, adds, ok)
	}
}
func TestEmptyPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vrps.json")
	defaults := []Record{rec("192.0.2.0/24", 24, 1)}
	s, _ := Open(path, defaults)
	if e := s.Replace(nil); e != nil {
		t.Fatal(e)
	}
	s, e := Open(path, defaults)
	if e != nil {
		t.Fatal(e)
	}
	_, _, rs := s.State()
	if len(rs) != 0 {
		t.Fatal("empty dataset replaced by defaults")
	}
}
