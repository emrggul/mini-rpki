package vrp

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type Record struct {
	Prefix netip.Prefix `json:"prefix"`
	MaxLen uint8        `json:"maxLength"`
	ASN    uint32       `json:"asn"`
}

func (r Record) Validate() error {
	if !r.Prefix.IsValid() || r.Prefix.Addr().Is4In6() || r.Prefix != r.Prefix.Masked() {
		return fmt.Errorf("prefix must be a canonical IPv4 or IPv6 network")
	}
	if int(r.MaxLen) < r.Prefix.Bits() || int(r.MaxLen) > r.Prefix.Addr().BitLen() {
		return fmt.Errorf("invalid maximum prefix length")
	}
	return nil
}
func Normalize(rs []Record) ([]Record, error) {
	set := map[Record]bool{}
	out := make([]Record, 0, len(rs))
	for _, r := range rs {
		if err := r.Validate(); err != nil {
			return nil, err
		}
		if !set[r] {
			out = append(out, r)
			set[r] = true
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Prefix.String() != b.Prefix.String() {
			return a.Prefix.String() < b.Prefix.String()
		}
		if a.MaxLen != b.MaxLen {
			return a.MaxLen < b.MaxLen
		}
		return a.ASN < b.ASN
	})
	return out, nil
}

type Snapshot struct {
	Serial  uint32
	Records []Record
}
type Store struct {
	mu      sync.RWMutex
	session uint16
	serial  uint32
	records []Record
	history []Snapshot
	path    string
}

func Open(path string, defaults []Record) (*Store, error) {
	var seed [2]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, err
	}
	s := &Store{session: binary.BigEndian.Uint16(seed[:]), serial: 1, path: path}
	rs := defaults
	if path != "" {
		b, err := os.ReadFile(path)
		if err == nil {
			if err = json.Unmarshal(b, &rs); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	var err error
	s.records, err = Normalize(rs)
	if err != nil {
		return nil, err
	}
	s.history = []Snapshot{{s.serial, append([]Record(nil), s.records...)}}
	return s, nil
}
func (s *Store) State() (uint16, uint32, []Record) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.session, s.serial, append([]Record(nil), s.records...)
}
func (s *Store) Replace(rs []Record) error { s.mu.Lock(); defer s.mu.Unlock(); return s.replace(rs) }
func (s *Store) Edit(r Record, add bool) error {
	if err := r.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rs := append([]Record(nil), s.records...)
	found := -1
	for i, v := range rs {
		if v == r {
			found = i
			break
		}
	}
	if add {
		if found >= 0 {
			return nil
		}
		rs = append(rs, r)
	} else {
		if found < 0 {
			return fmt.Errorf("exact VRP not found")
		}
		rs = append(rs[:found], rs[found+1:]...)
	}
	return s.replace(rs)
}
func (s *Store) replace(rs []Record) error {
	rs, err := Normalize(rs)
	if err != nil {
		return err
	}
	equal := len(rs) == len(s.records)
	if equal {
		for i := range rs {
			if rs[i] != s.records[i] {
				equal = false
				break
			}
		}
	}
	if equal {
		return nil
	}
	if s.path != "" {
		if err = os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
			return err
		}
		b, _ := json.MarshalIndent(rs, "", "  ")
		f, e := os.CreateTemp(filepath.Dir(s.path), ".vrps-*")
		if e != nil {
			return e
		}
		if e = f.Chmod(0644); e != nil {
			f.Close()
			return e
		}
		name := f.Name()
		defer os.Remove(name)
		if _, e = f.Write(b); e == nil {
			e = f.Sync()
		}
		ce := f.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
		if e = os.Rename(name, s.path); e != nil {
			return e
		}
	}
	old := make(map[Record]bool, len(s.records))
	next := make(map[Record]bool, len(rs))
	for _, r := range s.records {
		old[r] = true
	}
	for _, r := range rs {
		next[r] = true
	}
	s.serial++
	for _, r := range s.records {
		if !next[r] {
			log.Printf("VRP removed prefix=%s maxlen=%d asn=%d serial=%d", r.Prefix, r.MaxLen, r.ASN, s.serial)
		}
	}
	for _, r := range rs {
		if !old[r] {
			log.Printf("VRP added prefix=%s maxlen=%d asn=%d serial=%d", r.Prefix, r.MaxLen, r.ASN, s.serial)
		}
	}
	log.Printf("cache serial advanced serial=%d records=%d", s.serial, len(rs))
	s.records = rs
	s.history = append(s.history, Snapshot{s.serial, append([]Record(nil), rs...)})
	if len(s.history) > 129 {
		s.history = s.history[1:]
	}
	return nil
}

// Delta compares snapshots so add/remove cycles never emit duplicate announcements.
func (s *Store) Delta(session uint16, serial uint32) (uint16, uint32, []Record, []Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if session != s.session {
		return s.session, s.serial, nil, nil, false
	}
	for _, h := range s.history {
		if h.Serial == serial {
			old, now := map[Record]bool{}, map[Record]bool{}
			for _, r := range h.Records {
				old[r] = true
			}
			for _, r := range s.records {
				now[r] = true
			}
			var adds, dels []Record
			for _, r := range h.Records {
				if !now[r] {
					dels = append(dels, r)
				}
			}
			for _, r := range s.records {
				if !old[r] {
					adds = append(adds, r)
				}
			}
			return s.session, s.serial, adds, dels, true
		}
	}
	return s.session, s.serial, nil, nil, false
}
func ValidateRoute(rs []Record, p netip.Prefix, asn uint32) string {
	state := "Not Found"
	for _, r := range rs {
		if r.Prefix.Addr().BitLen() == p.Addr().BitLen() && r.Prefix.Bits() <= p.Bits() && r.Prefix.Contains(p.Addr()) {
			state = "Invalid"
			if r.ASN != 0 && r.ASN == asn && p.Bits() <= int(r.MaxLen) {
				return "Valid"
			}
		}
	}
	return state
}
