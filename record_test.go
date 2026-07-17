package bunny

import (
	"reflect"
	"testing"
	"time"

	"github.com/libdns/libdns"
)

func TestRecordToWireFormat(t *testing.T) {
	record := Record{
		Name:                   "www",
		TTL:                    300 * time.Second,
		Type:                   "A",
		Data:                   "1.2.3.4",
		Disabled:               true,
		GeolocationLatitude:    48.2,
		GeolocationLongitude:   16.4,
		LatencyZone:            "DE",
		SmartRoutingType:       SmartRoutingLatency,
		MonitorType:            MonitorPing,
		Accelerated:            true,
		AcceleratedPullZoneID:  11,
		PullZoneID:             22,
		ScriptID:               33,
		EnvironmentalVariables: map[string]string{"B": "2", "A": "1"},
	}

	wire, err := testZone.bunnyRecord(record)
	if err != nil {
		t.Fatal(err)
	}

	want := bunnyRecord{
		Type:                  bunnyTypeA,
		TTL:                   300,
		Name:                  "www",
		Value:                 "1.2.3.4",
		Disabled:              true,
		GeolocationLatitude:   48.2,
		GeolocationLongitude:  16.4,
		LatencyZone:           "DE",
		SmartRoutingType:      SmartRoutingLatency,
		MonitorType:           MonitorPing,
		Accelerated:           true,
		AcceleratedPullZoneID: 11,
		PullZoneID:            22,
		ScriptID:              33,
		EnviromentalVariables: []bunnyEnvVar{{Name: "A", Value: "1"}, {Name: "B", Value: "2"}},
	}
	if !reflect.DeepEqual(wire, want) {
		t.Errorf("got wire record %+v, want %+v", wire, want)
	}
}

func TestRecordFromWireFormat(t *testing.T) {
	wire := bunnyRecord{
		ID:                    7,
		Type:                  bunnyTypeA,
		TTL:                   300,
		Name:                  "www",
		Value:                 "1.2.3.4",
		Disabled:              true,
		MonitorType:           MonitorPing,
		MonitorStatus:         MonitorStatusOnline,
		EnviromentalVariables: []bunnyEnvVar{{Name: "A", Value: "1"}},
	}

	record, err := testZone.libdnsRecord(wire)
	if err != nil {
		t.Fatal(err)
	}

	want := Record{
		Name:                   "www",
		TTL:                    300 * time.Second,
		Type:                   "A",
		Data:                   "1.2.3.4",
		Disabled:               true,
		MonitorType:            MonitorPing,
		MonitorStatus:          MonitorStatusOnline,
		EnvironmentalVariables: map[string]string{"A": "1"},
	}
	if !reflect.DeepEqual(record, libdns.Record(want)) {
		t.Errorf("got record %+v, want %+v", record, want)
	}
}

// TestPlainRecordFromWireFormat verifies that records without Bunny-specific
// fields keep converting to the generic libdns types, exactly as before.
func TestPlainRecordFromWireFormat(t *testing.T) {
	wire := bunnyRecord{ID: 7, Type: bunnyTypeTXT, TTL: 300, Name: "test", Value: "hello"}

	record, err := testZone.libdnsRecord(wire)
	if err != nil {
		t.Fatal(err)
	}

	txt, ok := record.(libdns.TXT)
	if !ok {
		t.Fatalf("got record of type %T, want libdns.TXT", record)
	}
	if txt.Name != "test" || txt.Text != "hello" {
		t.Errorf("got record %+v, want Name=test Text=hello", txt)
	}
}
