package spaceship

import (
	"context"
	"fmt"
	"net/netip"
	"reflect"
	"strings"

	"github.com/libdns/libdns"
)

type recordProvider interface {
	GetRecords(context.Context, string) ([]libdns.Record, error)
	SetRecords(context.Context, string, []libdns.Record) ([]libdns.Record, error)
	DeleteRecords(context.Context, string, []libdns.Record) ([]libdns.Record, error)
}

type recordKey struct{ name, typ string }

// ownerKey accepts the relative and fully qualified names used by libdns.
func ownerKey(name, zone, typ string) recordKey {
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	zone = strings.ToLower(strings.TrimSuffix(zone, "."))
	if name == "" || name == "@" || name == zone {
		name = "@"
	} else if strings.HasSuffix(name, "."+zone) && zone != "" {
		name = strings.TrimSuffix(name, "."+zone)
	}
	return recordKey{name: name, typ: strings.ToUpper(typ)}
}

func nilRecord(r libdns.Record) bool {
	if r == nil {
		return true
	}
	v := reflect.ValueOf(r)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// SetRecords replaces an existing single A or AAAA record when its address
// changes. Ambiguous record sets are rejected to avoid deleting unrelated data.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	if err := p.ensureProvider(); err != nil {
		return nil, err
	}
	p.setMu.Lock()
	defer p.setMu.Unlock()
	return setRecords(ctx, p.Provider, zone, records)
}

func setRecords(ctx context.Context, provider recordProvider, zone string, records []libdns.Record) ([]libdns.Record, error) {
	wanted := make(map[recordKey]netip.Addr)
	indexes := make(map[recordKey]int)
	for i, record := range records {
		if nilRecord(record) {
			return nil, fmt.Errorf("spaceship: nil record")
		}
		rr := record.RR()
		typ := strings.ToUpper(rr.Type)
		if typ != "A" && typ != "AAAA" {
			continue
		}
		ip, err := netip.ParseAddr(rr.Data)
		if err != nil || (typ == "A" && !ip.Is4()) || (typ == "AAAA" && (!ip.Is6() || ip.Is4In6())) {
			return nil, fmt.Errorf("spaceship: invalid %s address for %s", typ, rr.Name)
		}
		key := ownerKey(rr.Name, zone, typ)
		if _, exists := wanted[key]; exists {
			return nil, fmt.Errorf("spaceship: multiple desired %s records for %s", typ, rr.Name)
		}
		wanted[key] = ip
		indexes[key] = i
	}
	if len(wanted) == 0 {
		return provider.SetRecords(ctx, zone, records)
	}

	existing, err := provider.GetRecords(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("spaceship: list existing records: %w", err)
	}
	seen := make(map[recordKey]bool)
	updates := append([]libdns.Record(nil), records...)
	var obsolete []libdns.Record
	for _, record := range existing {
		if nilRecord(record) {
			continue
		}
		rr := record.RR()
		key := ownerKey(rr.Name, zone, rr.Type)
		ip, matched := wanted[key]
		if !matched {
			continue
		}
		if seen[key] {
			return nil, fmt.Errorf("spaceship: multiple %s records for %s; resolve them before updating", key.typ, key.name)
		}
		seen[key] = true
		old, err := netip.ParseAddr(rr.Data)
		if err != nil {
			return nil, fmt.Errorf("spaceship: invalid existing %s record for %s", key.typ, key.name)
		}
		index := indexes[key]
		incoming := updates[index].RR()
		if incoming.TTL == 0 {
			incoming.TTL = rr.TTL
			updates[index] = incoming
		}
		if old != ip {
			obsolete = append(obsolete, record)
		}
	}

	// Create the replacement first so an API failure does not remove the only
	// address. A failed delete is reported and can leave both addresses present.
	updated, err := provider.SetRecords(ctx, zone, updates)
	if err != nil {
		return nil, fmt.Errorf("spaceship: set records: %w", err)
	}
	if len(obsolete) > 0 {
		if _, err := provider.DeleteRecords(ctx, zone, obsolete); err != nil {
			return updated, fmt.Errorf("spaceship: new records created but old records could not be removed: %w", err)
		}
	}
	return updated, nil
}
