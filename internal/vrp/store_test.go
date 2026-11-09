package vrp

import (
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func rec(p string, m uint8, a uint32) Record { return Record{netip.MustParsePrefix(p), m, a} }
func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vrps.json")
	s, e := Open(path, nil)
	if e != nil {
		t.Fatal(e)
	}
	session, start, _ := s.State()
	r := rec("192.0.2.0/24", 24, 65001)
	if e = s.Edit(r, true); e != nil {
		t.Fatal(e)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatalf("persistence permissions=%#o, want 0644", info.Mode().Perm())
	}
	if e = s.Edit(r, true); e != nil {
		t.Fatal(e)
	}
	_, serial, adds, dels, ok := s.Delta(session, start)
	if !ok || serial != start+1 || len(adds) != 1 || len(dels) != 0 {
		t.Fatal(serial, adds, dels, ok)
	}
	loaded, e := Open(path, nil)
	if e != nil {
		t.Fatal(e)
	}
	_, _, rs := loaded.State()
	if len(rs) != 1 || rs[0] != r {
		t.Fatal(rs)
	}
	if e = s.Edit(r, false); e != nil {
		t.Fatal(e)
	}
	_, _, adds, dels, ok = s.Delta(session, serial)
	if !ok || len(adds) != 0 || len(dels) != 1 {
		t.Fatal(adds, dels)
	}
	if e = s.Edit(r, false); e == nil {
		t.Fatal("missing deletion accepted")
	}
	_, _, adds, dels, ok = s.Delta(session, start)
	if !ok || len(adds)+len(dels) != 0 {
		t.Fatal("cancelled change")
	}
	if _, _, _, _, ok = s.Delta(session+1, start); ok {
		t.Fatal("wrong session")
	}
	if _, _, _, _, ok = s.Delta(session, 999); ok {
		t.Fatal("future serial")
	}
}
func TestHistory(t *testing.T) {
	s, _ := Open("", nil)
	id, start, _ := s.State()
	for i := uint32(1); i < 140; i++ {
		if e := s.Replace([]Record{rec("192.0.2.0/24", 24, i)}); e != nil {
			t.Fatal(e)
		}
	}
	if _, _, _, _, ok := s.Delta(id, start); ok {
		t.Fatal("old history retained")
	}
}
func TestValidation(t *testing.T) {
	for _, r := range []Record{rec("192.0.2.0/24", 23, 1), rec("192.0.2.0/24", 33, 1), rec("192.0.2.1/24", 24, 1), rec("2001:db8::/32", 129, 1), {}} {
		if r.Validate() == nil {
			t.Fatal(r)
		}
	}
	for _, r := range []Record{rec("0.0.0.0/0", 32, 0), rec("::/0", 128, 4294967295)} {
		if e := r.Validate(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestConcurrent(t *testing.T) {
	s, _ := Open("", nil)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := rec("192.0.2.0/24", 24, uint32(i))
			for j := 0; j < 20; j++ {
				s.Edit(r, true)
				id, serial, _ := s.State()
				s.Delta(id, serial)
				s.Edit(r, false)
			}
		}(i)
	}
	wg.Wait()
}
func TestPersistenceFailure(t *testing.T) {
	s, _ := Open("", nil)
	s.path = t.TempDir()
	_, serial, _ := s.State()
	if e := s.Replace([]Record{rec("192.0.2.0/24", 24, 1)}); e == nil {
		t.Fatal("expected failure")
	}
	_, after, rs := s.State()
	if serial != after || len(rs) != 0 {
		t.Fatal("failed save changed cache")
	}
}
