package bunny

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/libdns/libdns"
)

// fakeAPI is an httptest-backed stand-in for the Bunny API. Handlers are
// registered on mux; every request is recorded in calls as "METHOD /path".
type fakeAPI struct {
	t     *testing.T
	mux   *http.ServeMux
	calls []string
}

func newFakeAPI(t *testing.T) *fakeAPI {
	return &fakeAPI{t: t, mux: http.NewServeMux()}
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	f.mux.ServeHTTP(w, r)
}

func (f *fakeAPI) provider() *Provider {
	server := httptest.NewServer(f)
	f.t.Cleanup(server.Close)
	return &Provider{AccessKey: "test-key", apiBase: server.URL}
}

func (f *fakeAPI) respondJSON(w http.ResponseWriter, body any) {
	f.t.Helper()
	if err := json.NewEncoder(w).Encode(body); err != nil {
		f.t.Fatalf("encoding response: %v", err)
	}
}

// serveZone makes zone lookups resolve the given zone. The returned pointer
// can be mutated by other handlers to simulate state changes.
func (f *fakeAPI) serveZone(zone bunnyZone) *bunnyZone {
	state := &zone
	f.mux.HandleFunc("GET /dnszone", func(w http.ResponseWriter, r *http.Request) {
		f.respondJSON(w, getAllZonesResponse{Zones: []bunnyZone{*state}})
	})
	return state
}

var testZone = bunnyZone{ID: 42, Domain: "example.com"}

func TestSetRecords(t *testing.T) {
	ttl := 300 * time.Second

	tests := []struct {
		name     string
		existing []bunnyRecord
		desired  []libdns.Record
		// wantCalls are the expected record API calls, in order.
		wantCalls []string
	}{
		{
			name:      "updates an existing record in place",
			existing:  []bunnyRecord{{ID: 7, Type: bunnyTypeTXT, TTL: 300, Name: "test", Value: "old"}},
			desired:   []libdns.Record{libdns.TXT{Name: "test", TTL: ttl, Text: "new"}},
			wantCalls: []string{"POST /dnszone/42/records/7"},
		},
		{
			name:      "creates a record that does not exist",
			desired:   []libdns.Record{libdns.TXT{Name: "test", TTL: ttl, Text: "new"}},
			wantCalls: []string{"PUT /dnszone/42/records"},
		},
		{
			name: "deletes surplus records of the same name and type",
			existing: []bunnyRecord{
				{ID: 7, Type: bunnyTypeTXT, TTL: 300, Name: "test", Value: "old_1"},
				{ID: 8, Type: bunnyTypeTXT, TTL: 300, Name: "test", Value: "old_2"},
			},
			desired:   []libdns.Record{libdns.TXT{Name: "test", TTL: ttl, Text: "new"}},
			wantCalls: []string{"POST /dnszone/42/records/7", "DELETE /dnszone/42/records/8"},
		},
		{
			name:      "leaves records of other names alone",
			existing:  []bunnyRecord{{ID: 9, Type: bunnyTypeTXT, TTL: 300, Name: "other", Value: "keep"}},
			desired:   []libdns.Record{libdns.TXT{Name: "test", TTL: ttl, Text: "new"}},
			wantCalls: []string{"PUT /dnszone/42/records"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newFakeAPI(t)
			api.serveZone(testZone)
			api.mux.HandleFunc("GET /dnszone/42", func(w http.ResponseWriter, r *http.Request) {
				api.respondJSON(w, getAllRecordsResponse{Records: tc.existing})
			})
			api.mux.HandleFunc("PUT /dnszone/42/records", func(w http.ResponseWriter, r *http.Request) {
				api.respondJSON(w, bunnyRecord{ID: 100, Type: bunnyTypeTXT, TTL: 300, Name: "test", Value: "new"})
			})
			api.mux.HandleFunc("POST /dnszone/42/records/{id}", func(w http.ResponseWriter, r *http.Request) {})
			api.mux.HandleFunc("DELETE /dnszone/42/records/{id}", func(w http.ResponseWriter, r *http.Request) {})

			records, err := api.provider().SetRecords(t.Context(), "example.com", tc.desired)
			if err != nil {
				t.Fatal(err)
			}

			if len(records) != len(tc.desired) {
				t.Errorf("got %d records back, want %d", len(records), len(tc.desired))
			}
			var recordCalls []string
			for _, call := range api.calls {
				if strings.Contains(call, "/records") {
					recordCalls = append(recordCalls, call)
				}
			}
			if !slices.Equal(recordCalls, tc.wantCalls) {
				t.Errorf("got record API calls %v, want %v", recordCalls, tc.wantCalls)
			}
		})
	}
}
