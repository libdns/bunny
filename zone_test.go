package bunny

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

var fullZoneModel = bunnyZone{
	ID:                            42,
	Domain:                        "example.com",
	DnsSecEnabled:                 true,
	NameserversDetected:           true,
	CustomNameserversEnabled:      true,
	Nameserver1:                   "ns1.example.com",
	Nameserver2:                   "ns2.example.com",
	SoaEmail:                      "hostmaster@example.com",
	LoggingEnabled:                true,
	LoggingIPAnonymizationEnabled: true,
	LogAnonymizationType:          1,
	DateCreated:                   "2026-06-25T08:21:23",
	DateModified:                  "2026-07-17T08:36:22",
}

func wantFullZone() Zone {
	return Zone{
		ID:                            42,
		Domain:                        "example.com",
		DNSSECEnabled:                 true,
		NameserversDetected:           true,
		CustomNameserversEnabled:      true,
		Nameserver1:                   "ns1.example.com",
		Nameserver2:                   "ns2.example.com",
		SoaEmail:                      "hostmaster@example.com",
		LoggingEnabled:                true,
		LoggingIPAnonymizationEnabled: true,
		LogAnonymizationType:          1,
		DateCreated:                   "2026-06-25T08:21:23",
		DateModified:                  "2026-07-17T08:36:22",
	}
}

func TestGetZone(t *testing.T) {
	api := newFakeAPI(t)
	api.serveZone(testZone)
	api.mux.HandleFunc("GET /dnszone/42", func(w http.ResponseWriter, r *http.Request) {
		api.respondJSON(w, fullZoneModel)
	})

	zone, err := api.provider().GetZone(t.Context(), "example.com")
	if err != nil {
		t.Fatal(err)
	}

	if zone != wantFullZone() {
		t.Errorf("got zone %+v, want %+v", zone, wantFullZone())
	}
}

func TestCreateZone(t *testing.T) {
	api := newFakeAPI(t)
	api.mux.HandleFunc("POST /dnszone", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if want := `{"Domain":"example.com"}`; strings.TrimSpace(string(body)) != want {
			t.Errorf("got request body %s, want %s", body, want)
		}
		api.respondJSON(w, fullZoneModel)
	})

	zone, err := api.provider().CreateZone(t.Context(), "example.com.")
	if err != nil {
		t.Fatal(err)
	}

	if zone != wantFullZone() {
		t.Errorf("got zone %+v, want %+v", zone, wantFullZone())
	}
}

func TestUpdateZone(t *testing.T) {
	api := newFakeAPI(t)
	api.serveZone(testZone)
	api.mux.HandleFunc("POST /dnszone/42", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// Unset fields must not be sent, so the API leaves them unchanged.
		if want := `{"SoaEmail":"new@example.com"}`; strings.TrimSpace(string(body)) != want {
			t.Errorf("got request body %s, want %s", body, want)
		}
		api.respondJSON(w, fullZoneModel)
	})
	provider := api.provider()

	email := "new@example.com"
	zone, err := provider.UpdateZone(t.Context(), "example.com", ZoneUpdate{SoaEmail: &email})
	if err != nil {
		t.Fatal(err)
	}

	if zone != wantFullZone() {
		t.Errorf("got zone %+v, want %+v", zone, wantFullZone())
	}
	if len(provider.zones) != 0 {
		t.Error("zone cache was not invalidated after update")
	}
}

func TestDeleteZone(t *testing.T) {
	api := newFakeAPI(t)
	api.serveZone(testZone)
	api.mux.HandleFunc("DELETE /dnszone/42", func(w http.ResponseWriter, r *http.Request) {})
	provider := api.provider()

	if err := provider.DeleteZone(t.Context(), "example.com"); err != nil {
		t.Fatal(err)
	}

	if got := api.countCalls("DELETE /dnszone/42"); got != 1 {
		t.Errorf("got %d zone DELETEs, want 1", got)
	}
	if len(provider.zones) != 0 {
		t.Error("zone cache was not invalidated after delete")
	}
}

func TestCheckZoneAvailability(t *testing.T) {
	tests := []struct {
		name      string
		available bool
		apiStatus int
		wantErr   bool
	}{
		{name: "available", available: true},
		{name: "not available", available: false},
		{name: "returns API errors", apiStatus: http.StatusInternalServerError, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newFakeAPI(t)
			api.mux.HandleFunc("POST /dnszone/checkavailability", func(w http.ResponseWriter, r *http.Request) {
				if tc.apiStatus != 0 {
					w.WriteHeader(tc.apiStatus)
					return
				}
				body, _ := io.ReadAll(r.Body)
				if want := `{"Name":"example.com"}`; strings.TrimSpace(string(body)) != want {
					t.Errorf("got request body %s, want %s", body, want)
				}
				fmt.Fprintf(w, `{"Available":%v}`, tc.available)
			})

			available, err := api.provider().CheckZoneAvailability(t.Context(), "example.com")

			if (err != nil) != tc.wantErr {
				t.Fatalf("got error %v, want error: %v", err, tc.wantErr)
			}
			if available != tc.available {
				t.Errorf("got available=%v, want %v", available, tc.available)
			}
		})
	}
}

func TestExportZone(t *testing.T) {
	zoneFile := "example.com. 300 IN TXT \"hello\"\n"

	api := newFakeAPI(t)
	api.serveZone(testZone)
	api.mux.HandleFunc("GET /dnszone/42/export", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, zoneFile)
	})

	data, err := api.provider().ExportZone(t.Context(), "example.com")
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != zoneFile {
		t.Errorf("got exported zone %q, want %q", data, zoneFile)
	}
}

func TestImportZone(t *testing.T) {
	zoneFile := "example.com. 300 IN TXT \"hello\"\n"

	api := newFakeAPI(t)
	api.serveZone(testZone)
	api.mux.HandleFunc("POST /dnszone/42/import", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != zoneFile {
			t.Errorf("got request body %q, want %q", body, zoneFile)
		}
		api.respondJSON(w, ZoneImportResult{RecordsSuccessful: 3, RecordsFailed: 1, RecordsSkipped: 2})
	})

	result, err := api.provider().ImportZone(t.Context(), "example.com", []byte(zoneFile))
	if err != nil {
		t.Fatal(err)
	}

	want := ZoneImportResult{RecordsSuccessful: 3, RecordsFailed: 1, RecordsSkipped: 2}
	if result != want {
		t.Errorf("got result %+v, want %+v", result, want)
	}
}

func TestGetZoneStatistics(t *testing.T) {
	api := newFakeAPI(t)
	api.serveZone(testZone)
	api.mux.HandleFunc("GET /dnszone/42/statistics", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{
			"TotalQueriesServed": 1000,
			"QueriesServedChart": {"2026-07-16T00:00:00": 600, "2026-07-17T00:00:00": 400},
			"QueriesByTypeChart": {"A": 900, "TXT": 100}
		}`)
	})

	stats, err := api.provider().GetZoneStatistics(t.Context(), "example.com")
	if err != nil {
		t.Fatal(err)
	}

	if stats.TotalQueriesServed != 1000 {
		t.Errorf("got TotalQueriesServed=%d, want 1000", stats.TotalQueriesServed)
	}
	if got := stats.QueriesServedChart["2026-07-16T00:00:00"]; got != 600 {
		t.Errorf("got QueriesServedChart entry %v, want 600", got)
	}
	if got := stats.QueriesByTypeChart["A"]; got != 900 {
		t.Errorf("got QueriesByTypeChart entry %v, want 900", got)
	}
}
