package bunny

import (
	"net/http"
	"testing"
)

func (f *fakeAPI) countCalls(call string) int {
	count := 0
	for _, c := range f.calls {
		if c == call {
			count++
		}
	}
	return count
}

func TestEnableDNSSEC(t *testing.T) {
	tests := []struct {
		name           string
		alreadyEnabled bool
		apiStatus      int
		wantErr        bool
		wantPosts      int
		wantInfo       DNSSECInfo
	}{
		{
			name:      "enables and returns DS record details",
			wantPosts: 1,
			wantInfo: DNSSECInfo{
				Enabled:    true,
				DSRecord:   "example.com. 3600 IN DS 12345 13 2 ABCDEF",
				Digest:     "ABCDEF",
				DigestType: "SHA256",
				Algorithm:  13,
				PublicKey:  "pubkey",
				KeyTag:     12345,
				Flags:      257,
			},
		},
		{
			name:           "skips the API call when already enabled",
			alreadyEnabled: true,
			wantPosts:      0,
			wantInfo:       DNSSECInfo{Enabled: true},
		},
		{
			name:      "returns API errors",
			apiStatus: http.StatusInternalServerError,
			wantErr:   true,
			wantPosts: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newFakeAPI(t)
			zone := testZone
			zone.DnsSecEnabled = tc.alreadyEnabled
			api.serveZone(zone)
			api.mux.HandleFunc("POST /dnszone/42/dnssec", func(w http.ResponseWriter, r *http.Request) {
				if tc.apiStatus != 0 {
					w.WriteHeader(tc.apiStatus)
					return
				}
				api.respondJSON(w, tc.wantInfo)
			})

			info, err := api.provider().EnableDNSSEC(t.Context(), "example.com.")

			if (err != nil) != tc.wantErr {
				t.Fatalf("got error %v, want error: %v", err, tc.wantErr)
			}
			if got := api.countCalls("POST /dnszone/42/dnssec"); got != tc.wantPosts {
				t.Errorf("got %d DNSSEC POSTs, want %d", got, tc.wantPosts)
			}
			if err == nil && info != tc.wantInfo {
				t.Errorf("got info %+v, want %+v", info, tc.wantInfo)
			}
		})
	}
}

func TestEnableDNSSEC_ZoneNotFound(t *testing.T) {
	api := newFakeAPI(t)
	api.mux.HandleFunc("GET /dnszone", func(w http.ResponseWriter, r *http.Request) {
		api.respondJSON(w, getAllZonesResponse{})
	})

	if _, err := api.provider().EnableDNSSEC(t.Context(), "missing.com"); err == nil {
		t.Fatal("expected an error for an unknown zone")
	}
}

// TestEnableDNSSEC_RefreshesZoneCache verifies that enabling DNSSEC drops the
// cached zone, so a second call sees the new state and skips the API call.
func TestEnableDNSSEC_RefreshesZoneCache(t *testing.T) {
	api := newFakeAPI(t)
	zone := api.serveZone(testZone)
	api.mux.HandleFunc("POST /dnszone/42/dnssec", func(w http.ResponseWriter, r *http.Request) {
		zone.DnsSecEnabled = true
		api.respondJSON(w, DNSSECInfo{Enabled: true, DSRecord: "ds"})
	})
	provider := api.provider()

	for i := 0; i < 2; i++ {
		if _, err := provider.EnableDNSSEC(t.Context(), "example.com"); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}

	if got := api.countCalls("POST /dnszone/42/dnssec"); got != 1 {
		t.Errorf("got %d DNSSEC POSTs, want 1", got)
	}
}

func TestDisableDNSSEC(t *testing.T) {
	tests := []struct {
		name      string
		apiStatus int
		wantErr   bool
	}{
		{name: "disables DNSSEC", apiStatus: http.StatusOK},
		{name: "returns API errors", apiStatus: http.StatusInternalServerError, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newFakeAPI(t)
			api.serveZone(testZone)
			api.mux.HandleFunc("DELETE /dnszone/42/dnssec", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.apiStatus)
			})

			err := api.provider().DisableDNSSEC(t.Context(), "example.com")

			if (err != nil) != tc.wantErr {
				t.Fatalf("got error %v, want error: %v", err, tc.wantErr)
			}
			if got := api.countCalls("DELETE /dnszone/42/dnssec"); got != 1 {
				t.Errorf("got %d DNSSEC DELETEs, want 1", got)
			}
		})
	}
}
