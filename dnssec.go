package bunny

import (
	"context"
	"fmt"
)

// DNSSECInfo describes the DNSSEC state of a zone and the DS record that
// must be published at the domain's registrar.
type DNSSECInfo struct {
	Enabled bool

	// DSRecord is the complete DS record to publish at the registrar.
	// Bunny.net does NOT publish it for you; see EnableDNSSEC.
	DSRecord   string `json:"DsRecord"`
	Digest     string
	DigestType string
	Algorithm  int
	PublicKey  string
	KeyTag     int
	Flags      int

	// DSConfigured reports whether Bunny has detected the DS record at the
	// domain's parent zone.
	DSConfigured bool `json:"DsConfigured"`
}

// EnableDNSSEC enables DNSSEC on the zone hosting the given domain. Bunny.net
// manages keys and signs the zone automatically, but it does NOT publish the
// DS record at the domain's registrar: the caller must publish the returned
// DS record details at the registrar (or parent zone) themselves, or DNSSEC
// validation will not work.
//
// If DNSSEC is already enabled on the zone, no API call is made and the
// returned info has only Enabled set; the DS record details are shown in the
// Bunny.net dashboard.
func (p *Provider) EnableDNSSEC(ctx context.Context, domain string) (DNSSECInfo, error) {
	zone, err := p.getZone(ctx, unFQDN(domain))
	if err != nil {
		return DNSSECInfo{}, err
	}
	if zone.DnsSecEnabled {
		p.log(fmt.Sprintf("DNSSEC already enabled on zone %s", zone.Domain))
		return DNSSECInfo{Enabled: true}, nil
	}

	p.log(fmt.Sprintf("enabling DNSSEC on zone %s", zone.Domain))

	info := DNSSECInfo{}
	if err := p.doJSON(ctx, "POST", p.zoneURL(zone, "/dnssec"), nil, &info); err != nil {
		return DNSSECInfo{}, err
	}
	p.forgetZone(zone.ID)

	return info, nil
}

// DisableDNSSEC disables DNSSEC on the zone hosting the given domain. Remove
// the DS record at the domain's registrar before calling this, or DNSSEC
// validation of the zone will fail.
func (p *Provider) DisableDNSSEC(ctx context.Context, domain string) error {
	zone, err := p.getZone(ctx, unFQDN(domain))
	if err != nil {
		return err
	}

	p.log(fmt.Sprintf("disabling DNSSEC on zone %s", zone.Domain))

	if err := p.doJSON(ctx, "DELETE", p.zoneURL(zone, "/dnssec"), nil, nil); err != nil {
		return err
	}
	p.forgetZone(zone.ID)

	return nil
}

// zoneURL builds the API URL for a zone-scoped endpoint, e.g. "/dnssec".
func (p *Provider) zoneURL(zone bunnyZone, suffix string) string {
	return fmt.Sprintf("%s/dnszone/%d%s", p.apiURL(), zone.ID, suffix)
}
