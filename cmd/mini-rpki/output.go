package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/minirpki/mini-rpki/internal/control"
	"github.com/minirpki/mini-rpki/internal/vrp"
)

func printRecords(w io.Writer, records []vrp.Record) {
	fmt.Fprintf(w, "Records served: %d\n", len(records))
	if len(records) == 0 {
		fmt.Fprintln(w, "No records configured.")
		return
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "PREFIX\tMAXLEN\tORIGIN")
	for _, r := range records {
		fmt.Fprintf(table, "%s\t%d\tAS%d\n", r.Prefix, r.MaxLen, r.ASN)
	}
	table.Flush()
}

func renderResponse(w io.Writer, action string, data []byte, jsonOutput bool) error {
	if jsonOutput {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, data, "", "  "); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, pretty.String())
		return err
	}
	switch action {
	case "interface":
		var info control.AddressInfo
		if err := json.Unmarshal(data, &info); err != nil {
			return err
		}
		fmt.Fprintf(w, "RTR listener: %s\n", info.Listener)
		table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "IP ADDRESS\tTCP PORT\tINTERFACE")
		for _, endpoint := range info.Endpoints {
			iface := endpoint.Interface
			if iface == "" {
				iface = "-"
			}
			fmt.Fprintf(table, "%s\t%s\t%s\n", endpoint.IP, endpoint.Port, iface)
		}
		table.Flush()
		fmt.Fprintln(w, "Addresses are inside this container; other networks need routing or port forwarding.")

	case "list":
		var records []vrp.Record
		if err := json.Unmarshal(data, &records); err != nil {
			return err
		}
		printRecords(w, records)
	case "status":
		var status control.Status
		if err := json.Unmarshal(data, &status); err != nil {
			return err
		}
		fmt.Fprintf(w, "MiniRPKI laboratory RTR cache\nSession ID: %d\nSerial: %d\nRecords served: %d\nConnected routers: %d\nRTR timers: refresh %ds, retry %ds, expire %ds\nSerial Notify limit: once per minute per connection\n", status.Session, status.Serial, status.Records, len(status.Clients), status.Refresh, status.Retry, status.Expire)
		if len(status.Clients) == 0 {
			fmt.Fprintln(w, "No routers connected.")
			break
		}
		table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "\nROUTER\tCONNECTED SINCE (UTC)")
		for _, client := range status.Clients {
			fmt.Fprintf(table, "%s\t%s\n", client.Address, client.Since.UTC().Format(time.RFC3339))
		}
		table.Flush()
	default:
		var result struct {
			Serial uint32     `json:"serial"`
			Record vrp.Record `json:"record"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}
		if action == "clear" {
			fmt.Fprintln(w, "All records cleared.")
		} else {
			verb := "Saved"
			if action == "remove" {
				verb = "Removed"
			}
			fmt.Fprintf(w, "%s: %s maxlen %d AS%d\n", verb, result.Record.Prefix, result.Record.MaxLen, result.Record.ASN)
		}
		fmt.Fprintf(w, "Cache serial: %d\n", result.Serial)
		fmt.Fprintln(w, "Routers update after their next successful RTR query.")
	}
	return nil
}
