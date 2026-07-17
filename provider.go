package bunny

import (
	"context"
	"strings"
	"sync"

	"github.com/libdns/libdns"
)

// Provider facilitates DNS record manipulation with Bunny.net
type Provider struct {
	// AccessKey is the Bunny.net API key - see https://docs.bunny.net/reference/bunnynet-api-overview
	AccessKey string                        `json:"access_key"`
	Debug     bool                          `json:"debug"`
	Logger    func(string, []libdns.Record) `json:"-"`

	zones   map[string]bunnyZone `json:"-"`
	zonesMu sync.Mutex

	// apiBase overrides the API base URL in tests.
	apiBase string
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, domain string) ([]libdns.Record, error) {
	zone, err := p.getZone(ctx, unFQDN(domain))
	if err != nil {
		return nil, err
	}

	records, err := p.getAllRecords(ctx, zone)
	if err != nil {
		return nil, err
	}

	return records, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, domain string, records []libdns.Record) ([]libdns.Record, error) {
	zone, err := p.getZone(ctx, unFQDN(domain))
	if err != nil {
		return nil, err
	}

	var appendedRecords []libdns.Record
	for _, record := range records {
		newRecord, err := p.createRecord(ctx, zone, record)
		if err != nil {
			return nil, err
		}
		appendedRecords = append(appendedRecords, newRecord)
	}

	return appendedRecords, nil
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, domain string, records []libdns.Record) ([]libdns.Record, error) {
	zone, err := p.getZone(ctx, unFQDN(domain))
	if err != nil {
		return nil, err
	}

	existing, err := p.getDNSRecords(ctx, zone)
	if err != nil {
		return nil, err
	}

	// Group the desired records by name and type, comparing names in the
	// zone-relative form the API uses.
	type recordKey struct {
		name string
		typ  int
	}
	desired := make(map[recordKey][]libdns.Record)
	keys := []recordKey{}
	for _, record := range records {
		wire, err := zone.bunnyRecord(record)
		if err != nil {
			return nil, err
		}
		key := recordKey{wire.Name, wire.Type}
		if _, ok := desired[key]; !ok {
			keys = append(keys, key)
		}
		desired[key] = append(desired[key], record)
	}

	existingByKey := make(map[recordKey][]bunnyRecord)
	for _, record := range existing {
		key := recordKey{record.Name, record.Type}
		existingByKey[key] = append(existingByKey[key], record)
	}

	// For each name+type, update existing records in place, delete surplus
	// ones and create the rest, so the zone ends up with exactly the desired
	// records for that name and type.
	setRecords := []libdns.Record{}
	for _, key := range keys {
		olds, news := existingByKey[key], desired[key]

		updates := min(len(olds), len(news))
		for i := 0; i < updates; i++ {
			if err := p.updateRecord(ctx, zone, news[i], olds[i].ID); err != nil {
				return nil, err
			}
			setRecords = append(setRecords, news[i])
		}
		for _, old := range olds[updates:] {
			if err := p.deleteRecordByID(ctx, zone, old.ID); err != nil {
				return nil, err
			}
		}
		for _, new := range news[updates:] {
			created, err := p.createRecord(ctx, zone, new)
			if err != nil {
				return nil, err
			}
			setRecords = append(setRecords, created)
		}
	}

	return setRecords, nil
}

// DeleteRecords deletes the records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, domain string, records []libdns.Record) ([]libdns.Record, error) {
	zone, err := p.getZone(ctx, unFQDN(domain))
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		err := p.deleteRecord(ctx, zone, record)
		if err != nil {
			return nil, err
		}
	}

	return records, nil
}

// ListZones returns the list of available DNS zones.
func (p *Provider) ListZones(ctx context.Context) ([]libdns.Zone, error) {
	bunnyZones, err := p.getAllZones(ctx)
	if err != nil {
		return nil, err
	}

	zones := make([]libdns.Zone, len(bunnyZones))
	for i, bunnyZone := range bunnyZones {
		zones[i] = libdns.Zone{
			Name: bunnyZone.Domain,
		}
	}

	return zones, nil
}

// unFQDN trims any trailing "." from fqdn. Bunny.net's API does not use FQDNs.
func unFQDN(fqdn string) string {
	return strings.TrimSuffix(fqdn, ".")
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
	_ libdns.ZoneLister     = (*Provider)(nil)
)
