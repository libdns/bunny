package bunny

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/libdns/libdns"
	"golang.org/x/net/publicsuffix"
)

type getAllRecordsResponse struct {
	Records []bunnyRecord `json:"Records"`
}

type getAllZonesResponse struct {
	Zones []bunnyZone `json:"Items"`
}

type bunnyZone struct {
	ID     int    `json:"Id"`
	Domain string `json:"Domain"`
}

type bunnyRecord struct {
	ID    int    `json:"Id,omitempty"`
	Type  int    `json:"Type"`
	Name  string `json:"Name"`
	Value string `json:"Value"`
	TTL   int    `json:"Ttl"`
}

func (p *Provider) doRequest(request *http.Request) ([]byte, error) {
	request.Header.Add("accept", "application/json")
	request.Header.Add("AccessKey", p.AccessKey)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("%s (%d)", http.StatusText(response.StatusCode), response.StatusCode)
	}

	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (p *Provider) getZoneID(ctx context.Context, domain string) (int, error) {
	if domain == "" {
		return 0, fmt.Errorf("domain is an empty string")
	}

	p.log(fmt.Sprintf("fetching zone ID for %s", domain))

	// Get all possible parent domains to check
	zoneGuesses := getBaseDomainNameGuesses(domain)

	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://api.bunny.net/dnszone", nil)
	if err != nil {
		return 0, err
	}

	data, err := p.doRequest(req)
	if err != nil {
		return 0, err
	}

	result := getAllZonesResponse{}
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, err
	}

	// Iterate through domain guesses (most specific to least specific)
	for _, zoneGuess := range zoneGuesses {
		for _, zone := range result.Zones {
			if strings.EqualFold(zone.Domain, zoneGuess) {
				p.log(fmt.Sprintf("found zone ID %d for %s using name %s",
					zone.ID, domain, zone.Domain))
				return zone.ID, nil
			}
		}
	}

	return 0, fmt.Errorf("zone not found for domain: %s", domain)
}

// getBaseDomainNameGuesses returns a slice of possible parent domain names ordered
// from most specific to least specific. For a domain "sub.example.com", it returns
// ["sub.example.com", "example.com", "com"]. If the input domain has fewer than
// 2 parts (no dots or just one dot), it returns a slice containing only the input
// domain.
func getBaseDomainNameGuesses(domain string) []string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return []string{domain}
	}

	var guesses []string
	for i := 0; i < len(parts)-1; i++ {
		guess := strings.Join(parts[i:], ".")
		guesses = append(guesses, guess)
	}
	return guesses
}

func (p *Provider) getAllRecords(ctx context.Context, domain string) ([]libdns.Record, error) {
	p.log(fmt.Sprintf("fetching all records for %s", domain))

	domain = strings.ToLower(domain)
	zone, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		zone = domain
	}

	suffix := "." + zone
	var subdomain string
	if strings.HasSuffix(domain, suffix) {
		subdomain = strings.TrimSuffix(domain, suffix)
	}

	zoneID, err := p.getZoneID(ctx, zone)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://api.bunny.net/dnszone/%d", zoneID), nil)
	if err != nil {
		return nil, err
	}

	data, err := p.doRequest(req)
	if err != nil {
		return nil, err
	}

	result := getAllRecordsResponse{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	records := []libdns.Record{}
	for _, resData := range result.Records {
		if subdomain != "" {
			resName := strings.ToLower(resData.Name)
			// in case of a subdomain, we need to filter the records by name
			if resName != subdomain && !strings.HasSuffix(resName, "."+subdomain) {
				continue
			}
		}
		records = append(records, libdns.Record{
			ID:    fmt.Sprint(resData.ID),
			Type:  fromBunnyType(resData.Type),
			Name:  resData.Name,
			Value: resData.Value,
			TTL:   time.Duration(resData.TTL) * time.Second,
		})
	}

	p.log(fmt.Sprintf("done fetching %d record(s) in zone %s", len(records), zone), records...)

	return records, nil
}

func (p *Provider) createRecord(ctx context.Context, zone string, record libdns.Record) (libdns.Record, error) {
	p.log(fmt.Sprintf("creating %s record in zone %s", record.Type, zone), record)

	zoneID, err := p.getZoneID(ctx, zone)
	if err != nil {
		return libdns.Record{}, err
	}

	reqData := bunnyRecord{
		Type:  toBunnyType(record.Type),
		Name:  record.Name,
		Value: record.Value,
		TTL:   int(record.TTL.Seconds()),
	}

	reqBuffer, err := json.Marshal(reqData)
	if err != nil {
		return libdns.Record{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("https://api.bunny.net/dnszone/%d/records", zoneID), bytes.NewBuffer(reqBuffer))
	if err != nil {
		return libdns.Record{}, err
	}

	req.Header.Add("content-type", "application/json")
	data, err := p.doRequest(req)
	if err != nil {
		return libdns.Record{}, err
	}

	result := bunnyRecord{}
	if err := json.Unmarshal(data, &result); err != nil {
		return libdns.Record{}, err
	}

	resRecord := libdns.Record{
		ID:    fmt.Sprint(result.ID),
		Type:  fromBunnyType(result.Type),
		Name:  libdns.RelativeName(result.Name, zone),
		Value: result.Value,
		TTL:   time.Duration(result.TTL) * time.Second,
	}

	p.log(fmt.Sprintf("done creating %s record %s in zone %s", resRecord.Type, resRecord.ID, zone), resRecord)

	return resRecord, nil
}

func (p *Provider) deleteRecord(ctx context.Context, zone string, record libdns.Record) error {
	p.log(fmt.Sprintf("deleting %s record in zone %s", record.Type, zone), record)

	zoneID, err := p.getZoneID(ctx, zone)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("https://api.bunny.net/dnszone/%d/records/%s", zoneID, url.PathEscape(record.ID)), nil)
	if err != nil {
		return err
	}

	_, err = p.doRequest(req)
	if err != nil {
		return err
	}

	p.log(fmt.Sprintf("done deleting %s record %s in zone %s", record.Type, record.ID, zone), record)

	return nil
}

func (p *Provider) updateRecord(ctx context.Context, zone string, record libdns.Record) error {
	p.log(fmt.Sprintf("updating %s record in zone %s", record.Type, zone), record)

	zoneID, err := p.getZoneID(ctx, zone)
	if err != nil {
		return err
	}

	reqData := bunnyRecord{
		Type:  toBunnyType(record.Type),
		Name:  record.Name,
		Value: record.Value,
		TTL:   int(record.TTL.Seconds()),
	}

	reqBuffer, err := json.Marshal(reqData)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://api.bunny.net/dnszone/%d/records/%s", zoneID, url.PathEscape(record.ID)), bytes.NewBuffer(reqBuffer))
	if err != nil {
		return err
	}

	req.Header.Add("content-type", "application/json")

	_, err = p.doRequest(req)
	if err != nil {
		return err
	}

	p.log(fmt.Sprintf("done updating %s record %s in zone %s", record.Type, record.ID, zone), record)

	return nil
}

// Creates a new record if it does not exist, or updates an existing one.
func (p *Provider) createOrUpdateRecord(ctx context.Context, zone string, record libdns.Record) (libdns.Record, error) {
	if record.ID == "" {
		return p.createRecord(ctx, zone, record)
	}

	err := p.updateRecord(ctx, zone, record)
	return record, err
}

func (p *Provider) log(msg string, records ...libdns.Record) {
	if p.Logger != nil {
		p.Logger(msg, records)
	} else if p.Debug {
		fmt.Printf("[bunny] %s\n", msg)
		for _, record := range records {
			var id string
			if record.ID == "" {
				id = "(new)"
			} else {
				id = record.ID
			}
			fmt.Printf("[bunny]   %s: ID=%s, TTL=%s, Priority=%d, Name=%s, Value=%s\n",
				record.Type, id, record.TTL, record.Priority, record.Name, record.Value)
		}
	}
}

const (
	// The Bunny.net API uses integers to represent record types.
	bunnyTypeA        = 0
	bunnyTypeAAAA     = 1
	bunnyTypeCNAME    = 2
	bunnyTypeTXT      = 3
	bunnyTypeMX       = 4
	bunnyTypeRedirect = 5
	bunnyTypeFlatten  = 6
	bunnyTypePullZone = 7
	bunnyTypeSRV      = 8
	bunnyTypeCAA      = 9
	bunnyTypePTR      = 10
	bunnyTypeScript   = 11
	bunnyTypeNS       = 12
)

// Converts the Bunny.net record type to the libdns record type.
func fromBunnyType(t int) string {
	switch t {
	case bunnyTypeA:
		return "A"
	case bunnyTypeAAAA:
		return "AAAA"
	case bunnyTypeCNAME:
		return "CNAME"
	case bunnyTypeTXT:
		return "TXT"
	case bunnyTypeMX:
		return "MX"
	case bunnyTypeRedirect:
		return "Redirect"
	case bunnyTypeFlatten:
		return "Flatten"
	case bunnyTypePullZone:
		return "PullZone"
	case bunnyTypeSRV:
		return "SRV"
	case bunnyTypeCAA:
		return "CAA"
	case bunnyTypePTR:
		return "PTR"
	case bunnyTypeScript:
		return "Script"
	case bunnyTypeNS:
		return "NS"
	default:
		panic(fmt.Sprintf("unknown record type: %d", t))
	}
}

// Converts the libdns record type to the Bunny.net record type.
func toBunnyType(t string) int {
	switch t {
	case "A":
		return bunnyTypeA
	case "AAAA":
		return bunnyTypeAAAA
	case "CNAME":
		return bunnyTypeCNAME
	case "TXT":
		return bunnyTypeTXT
	case "MX":
		return bunnyTypeMX
	case "Redirect":
		return bunnyTypeRedirect
	case "Flatten":
		return bunnyTypeFlatten
	case "PullZone":
		return bunnyTypePullZone
	case "SRV":
		return bunnyTypeSRV
	case "CAA":
		return bunnyTypeCAA
	case "PTR":
		return bunnyTypePTR
	case "Script":
		return bunnyTypeScript
	case "NS":
		return bunnyTypeNS
	default:
		panic(fmt.Sprintf("unknown record type: %s", t))
	}
}
