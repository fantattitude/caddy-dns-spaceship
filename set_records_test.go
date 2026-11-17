package spaceship

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/libdns/libdns"
)

type fakeRecords struct {
	existing          []libdns.Record
	calls             []string
	setErr, deleteErr error
}

func (f *fakeRecords) GetRecords(context.Context, string) ([]libdns.Record, error) {
	f.calls = append(f.calls, "get")
	return f.existing, nil
}
func (f *fakeRecords) SetRecords(_ context.Context, _ string, records []libdns.Record) ([]libdns.Record, error) {
	f.calls = append(f.calls, "set")
	return records, f.setErr
}
func (f *fakeRecords) DeleteRecords(_ context.Context, _ string, records []libdns.Record) ([]libdns.Record, error) {
	f.calls = append(f.calls, "delete")
	return records, f.deleteErr
}
func addr(name, ip string) libdns.Record {
	return libdns.Address{Name: name, IP: netip.MustParseAddr(ip)}
}
func TestSetRecordsAddressUpdate(t *testing.T) {
	cases := []struct {
		name     string
		existing []libdns.Record
		want     []libdns.Record
		calls    string
		fail     bool
	}{
		{"create", nil, []libdns.Record{addr("@", "2001:db8::2")}, "get,set", false},
		{"replace", []libdns.Record{addr("example.com.", "2001:db8::1"), addr("@", "1.1.1.1")}, []libdns.Record{addr("@", "2001:db8::2")}, "get,set,delete", false},
		{"unchanged", []libdns.Record{addr("@", "2001:db8::2")}, []libdns.Record{addr("example.com.", "2001:db8::2")}, "get,set", false},
		{"ambiguous", []libdns.Record{addr("@", "2001:db8::1"), addr("@", "2001:db8::3")}, []libdns.Record{addr("@", "2001:db8::2")}, "get", true},
		{"multiple desired", nil, []libdns.Record{addr("@", "2001:db8::1"), addr("example.com.", "2001:db8::2")}, "", true},
		{"txt unchanged", nil, []libdns.Record{libdns.TXT{Name: "_acme-challenge", Text: "value"}}, "set", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRecords{existing: tc.existing}
			_, err := setRecords(context.Background(), f, "example.com.", tc.want)
			if (err != nil) != tc.fail {
				t.Fatalf("error: %v", err)
			}
			if calls := strings.Join(f.calls, ","); calls != tc.calls {
				t.Fatalf("calls %s, want %s", calls, tc.calls)
			}
		})
	}
}
func TestSetRecordsFailures(t *testing.T) {
	existing := []libdns.Record{addr("@", "2001:db8::1")}
	wanted := []libdns.Record{addr("@", "2001:db8::2")}
	f := &fakeRecords{existing: existing, setErr: errors.New("unavailable")}
	if _, err := setRecords(context.Background(), f, "example.com.", wanted); err == nil || len(f.calls) != 2 {
		t.Fatal("failed write must not delete existing record")
	}
	f = &fakeRecords{existing: existing, deleteErr: errors.New("unavailable")}
	if _, err := setRecords(context.Background(), f, "example.com.", wanted); err == nil || len(f.calls) != 3 {
		t.Fatal("failed delete must be reported")
	}
}

func TestTTLHandling(t *testing.T) {
	for _, tc := range []struct {
		name      string
		existing  []libdns.Record
		requested time.Duration
		want      time.Duration
	}{
		{"preserve", []libdns.Record{libdns.RR{Name: "@", Type: "A", Data: "1.1.1.1", TTL: 30 * time.Minute}}, 0, 30 * time.Minute},
		{"override", []libdns.Record{libdns.RR{Name: "@", Type: "A", Data: "1.1.1.1", TTL: 30 * time.Minute}}, 5 * time.Minute, 5 * time.Minute},
		{"new", nil, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := libdns.RR{Name: "@", Type: "A", Data: "8.8.8.8", TTL: tc.requested}
			f := &fakeRecords{existing: tc.existing}
			updated, err := setRecords(context.Background(), f, "example.com.", []libdns.Record{record})
			if err != nil || len(updated) != 1 || updated[0].RR().TTL != tc.want {
				t.Fatalf("updated %v, error %v", updated, err)
			}
		})
	}
}
