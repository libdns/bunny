package bunny

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Zone describes a DNS zone hosted on Bunny.net, including settings that are
// not part of the generic libdns.Zone.
type Zone struct {
	ID     int64
	Domain string

	// DNSSECEnabled reports whether Bunny signs the zone. See EnableDNSSEC
	// for the DS record that must additionally be published at the registrar.
	DNSSECEnabled bool

	NameserversDetected      bool
	CustomNameserversEnabled bool
	Nameserver1              string
	Nameserver2              string
	SoaEmail                 string

	LoggingEnabled                bool
	LoggingIPAnonymizationEnabled bool
	LogAnonymizationType          int

	// Timestamps as returned by the Bunny API, e.g. "2026-06-25T08:21:23"
	// (RFC 3339 without a time zone; times are UTC).
	DateCreated  string
	DateModified string
}

// ZoneUpdate describes changes to a zone's settings for UpdateZone.
// Nil fields are left unchanged.
type ZoneUpdate struct {
	CustomNameserversEnabled      *bool   `json:"CustomNameserversEnabled,omitempty"`
	Nameserver1                   *string `json:"Nameserver1,omitempty"`
	Nameserver2                   *string `json:"Nameserver2,omitempty"`
	SoaEmail                      *string `json:"SoaEmail,omitempty"`
	LoggingEnabled                *bool   `json:"LoggingEnabled,omitempty"`
	LoggingIPAnonymizationEnabled *bool   `json:"LoggingIPAnonymizationEnabled,omitempty"`
	LogAnonymizationType          *int    `json:"LogAnonymizationType,omitempty"`
}

// ZoneStatistics reports DNS query statistics for a zone over the last 30
// days. Chart keys are timestamps as returned by the API.
type ZoneStatistics struct {
	TotalQueriesServed       int64
	QueriesServedChart       map[string]float64
	NormalQueriesServedChart map[string]float64
	SmartQueriesServedChart  map[string]float64
	QueriesByTypeChart       map[string]int64
}

// ZoneImportResult reports the outcome of an ImportZone call.
type ZoneImportResult struct {
	RecordsSuccessful int
	RecordsFailed     int
	RecordsSkipped    int
}

// GetZone returns the zone hosting the given domain, with its settings.
func (p *Provider) GetZone(ctx context.Context, domain string) (Zone, error) {
	zone, err := p.getZone(ctx, unFQDN(domain))
	if err != nil {
		return Zone{}, err
	}

	fresh := bunnyZone{}
	if err := p.doJSON(ctx, "GET", p.zoneURL(zone, ""), nil, &fresh); err != nil {
		return Zone{}, err
	}

	return fresh.zone(), nil
}

// CreateZone creates a new zone for the given domain and returns it.
func (p *Provider) CreateZone(ctx context.Context, domain string) (Zone, error) {
	p.log(fmt.Sprintf("creating zone %s", domain))

	body := struct{ Domain string }{unFQDN(domain)}
	created := bunnyZone{}
	if err := p.doJSON(ctx, "POST", p.apiURL()+"/dnszone", body, &created); err != nil {
		return Zone{}, err
	}

	p.log(fmt.Sprintf("done creating zone %s with ID %d", created.Domain, created.ID))

	return created.zone(), nil
}

// UpdateZone changes the settings of the zone hosting the given domain and
// returns the updated zone.
func (p *Provider) UpdateZone(ctx context.Context, domain string, update ZoneUpdate) (Zone, error) {
	zone, err := p.getZone(ctx, unFQDN(domain))
	if err != nil {
		return Zone{}, err
	}

	p.log(fmt.Sprintf("updating zone %s", zone.Domain))

	updated := bunnyZone{}
	if err := p.doJSON(ctx, "POST", p.zoneURL(zone, ""), update, &updated); err != nil {
		return Zone{}, err
	}
	p.forgetZone(zone.ID)

	return updated.zone(), nil
}

// DeleteZone deletes the zone hosting the given domain and all its records.
func (p *Provider) DeleteZone(ctx context.Context, domain string) error {
	zone, err := p.getZone(ctx, unFQDN(domain))
	if err != nil {
		return err
	}

	p.log(fmt.Sprintf("deleting zone %s", zone.Domain))

	if err := p.doJSON(ctx, "DELETE", p.zoneURL(zone, ""), nil, nil); err != nil {
		return err
	}
	p.forgetZone(zone.ID)

	return nil
}

// CheckZoneAvailability reports whether a zone for the given domain can be
// added to the account.
func (p *Provider) CheckZoneAvailability(ctx context.Context, domain string) (bool, error) {
	body := struct{ Name string }{unFQDN(domain)}
	result := struct{ Available bool }{}
	if err := p.doJSON(ctx, "POST", p.apiURL()+"/dnszone/checkavailability", body, &result); err != nil {
		return false, err
	}

	return result.Available, nil
}

// ExportZone returns all records of the zone hosting the given domain as a
// BIND zone file.
func (p *Provider) ExportZone(ctx context.Context, domain string) ([]byte, error) {
	zone, err := p.getZone(ctx, unFQDN(domain))
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", p.zoneURL(zone, "/export"), nil)
	if err != nil {
		return nil, err
	}

	return p.doRequest(req)
}

// ImportZone imports records from a BIND zone file into the zone hosting the
// given domain and reports how many records were imported.
func (p *Provider) ImportZone(ctx context.Context, domain string, zoneFile []byte) (ZoneImportResult, error) {
	zone, err := p.getZone(ctx, unFQDN(domain))
	if err != nil {
		return ZoneImportResult{}, err
	}

	p.log(fmt.Sprintf("importing records into zone %s", zone.Domain))

	req, err := http.NewRequestWithContext(ctx, "POST", p.zoneURL(zone, "/import"), bytes.NewReader(zoneFile))
	if err != nil {
		return ZoneImportResult{}, err
	}
	req.Header.Add("content-type", "application/octet-stream")

	data, err := p.doRequest(req)
	if err != nil {
		return ZoneImportResult{}, err
	}

	result := ZoneImportResult{}
	if err := json.Unmarshal(data, &result); err != nil {
		return ZoneImportResult{}, err
	}

	p.log(fmt.Sprintf("done importing records into zone %s: %d successful, %d failed, %d skipped",
		zone.Domain, result.RecordsSuccessful, result.RecordsFailed, result.RecordsSkipped))

	return result, nil
}

// GetZoneStatistics returns DNS query statistics of the zone hosting the
// given domain for the last 30 days.
func (p *Provider) GetZoneStatistics(ctx context.Context, domain string) (ZoneStatistics, error) {
	zone, err := p.getZone(ctx, unFQDN(domain))
	if err != nil {
		return ZoneStatistics{}, err
	}

	stats := ZoneStatistics{}
	if err := p.doJSON(ctx, "GET", p.zoneURL(zone, "/statistics"), nil, &stats); err != nil {
		return ZoneStatistics{}, err
	}

	return stats, nil
}
