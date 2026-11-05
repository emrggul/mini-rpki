package main

import (
	"flag"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/minirpki/mini-rpki/internal/control"
	"github.com/minirpki/mini-rpki/internal/vrp"
)

func parseRequest(args []string) (control.Request, error) {
	// Preserve the old vrp prefix for existing scripts without advertising it.
	if len(args) > 0 && args[0] == "vrp" {
		args = args[1:]
	}
	q := control.Request{}
	if len(args) == 0 {
		return q, fmt.Errorf("use add, remove, list, clear, status, or interface")
	}
	q.Action = args[0]
	switch q.Action {
	case "status", "list", "clear", "interface":
		if len(args) != 1 {
			return q, fmt.Errorf("usage: mini-rpki %s", q.Action)
		}
	case "add", "remove":
		if len(args) < 2 {
			return q, fmt.Errorf("usage: mini-rpki %s PREFIX --asn ASN [--maxlen LENGTH]", q.Action)
		}
		prefix, err := netip.ParsePrefix(args[1])
		if err != nil {
			return q, err
		}
		flags := flag.NewFlagSet(q.Action, flag.ContinueOnError)
		asn := flags.String("asn", "", "origin ASN (0..4294967295)")
		maxlen := flags.String("maxlen", strconv.Itoa(prefix.Bits()), "maximum length (defaults to prefix length)")
		if err := flags.Parse(args[2:]); err != nil {
			return q, err
		}
		if flags.NArg() != 0 {
			return q, fmt.Errorf("unexpected arguments")
		}
		a, err := strconv.ParseUint(*asn, 10, 32)
		if err != nil {
			return q, fmt.Errorf("invalid or missing --asn")
		}
		m, err := strconv.ParseUint(*maxlen, 10, 8)
		if err != nil {
			return q, fmt.Errorf("invalid --maxlen")
		}
		q.Record = vrp.Record{Prefix: prefix, ASN: uint32(a), MaxLen: uint8(m)}
		if err := q.Record.Validate(); err != nil {
			return q, err
		}
	default:
		return q, fmt.Errorf("unknown command %q; use --help", q.Action)
	}
	return q, nil
}
