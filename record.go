package bunny

import (
	"time"

	"github.com/libdns/libdns"
)

// Record is a DNS record with Bunny.net-specific extensions that the generic
// libdns record types cannot represent. It implements libdns.Record and can
// be passed to AppendRecords/SetRecords/DeleteRecords like any other record;
// records read from a zone are returned as a Record whenever they carry any
// of these extensions, and as a generic libdns type otherwise.
type Record struct {
	// The standard record data, as in libdns.RR. Data is sent to the API
	// verbatim as the record value.
	Name string
	TTL  time.Duration
	Type string
	Data string

	// Disabled stops the record from being served without deleting it.
	Disabled bool

	// GeoDNS / smart routing (see SmartRoutingType constants).
	GeolocationLatitude  float64
	GeolocationLongitude float64
	LatencyZone          string
	SmartRoutingType     int

	// Health monitoring (see MonitorType constants). MonitorStatus is
	// reported by the API and ignored when writing.
	MonitorType   int
	MonitorStatus int

	// CDN acceleration through a Bunny Pull Zone.
	Accelerated           bool
	AcceleratedPullZoneID int64

	// Targets for PullZone- and Script-type records.
	PullZoneID int64
	ScriptID   int64

	// EnvironmentalVariables of Script-type records.
	EnvironmentalVariables map[string]string
}

// RR returns the standard part of the record. Implements libdns.Record.
func (r Record) RR() libdns.RR {
	return libdns.RR{
		Name: r.Name,
		TTL:  r.TTL,
		Type: r.Type,
		Data: r.Data,
	}
}

// SmartRoutingType values.
const (
	SmartRoutingNone        = 0
	SmartRoutingLatency     = 1
	SmartRoutingGeolocation = 2
)

// MonitorType values.
const (
	MonitorNone    = 0
	MonitorPing    = 1
	MonitorHTTP    = 2
	MonitorMonitor = 3
)

// MonitorStatus values.
const (
	MonitorStatusUnknown = 0
	MonitorStatusOnline  = 1
	MonitorStatusOffline = 2
)
