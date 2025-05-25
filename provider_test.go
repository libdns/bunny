package bunny_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/libdns/bunny"
	"github.com/libdns/libdns"
)

var (
	envAccessKey = ""
	envZone      = ""
	ttl          = time.Duration(120 * time.Second)
	testRecords  = []libdns.Record{
		libdns.TXT{
			Name: "test_1",
			TTL:  ttl,
			Text: "test_value_1",
		},
		libdns.TXT{
			Name: "test_2",
			TTL:  ttl,
			Text: "test_value_2",
		},
		libdns.TXT{
			Name: "test_3",
			TTL:  ttl,
			Text: "test_value_3",
		},
		libdns.CAA{
			Name:  "test_4",
			TTL:   ttl,
			Flags: 12,
			Tag:   "test",
			Value: "test_value_4",
		},
		libdns.MX{
			Name:       "test_5",
			TTL:        ttl,
			Preference: 10,
			Target:     "mx.example.com",
		},
		libdns.SRV{
			Service:   "sip",
			Transport: "tcp",
			Name:      "test_5",
			TTL:       ttl,
			Priority:  0,
			Weight:    5,
			Port:      5060,
			Target:    "sipserver.example.com",
		},
	}
)

type testRecordsCleanup = func(testRecords []libdns.Record)

func find[T any](elements []T, predicate func(T) bool) (T, bool) {
	for _, element := range elements {
		if predicate(element) {
			return element, true
		}
	}

	return *new(T), false
}

func contains[T any](elements []T, predicate func(T) bool) bool {
	_, ok := find(elements, predicate)

	return ok
}

func compareRecords(lhs libdns.Record, rhs libdns.Record) bool {
	return lhs.RR().Type == rhs.RR().Type &&
		lhs.RR().Name == rhs.RR().Name &&
		lhs.RR().Data == rhs.RR().Data &&
		lhs.RR().TTL == rhs.RR().TTL
}

func setupTestRecords(t *testing.T, p *bunny.Provider) ([]libdns.Record, testRecordsCleanup) {
	records, err := p.AppendRecords(context.Background(), envZone, testRecords)
	if err != nil {
		t.Fatal(err)
		return nil, func([]libdns.Record) {}
	}

	return records, func(testRecords []libdns.Record) {
		cleanupRecords(t, p, append(records, testRecords...))
	}
}

func cleanupRecords(t *testing.T, p *bunny.Provider, r []libdns.Record) {
	_, err := p.DeleteRecords(context.Background(), envZone, r)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
}

func TestMain(m *testing.M) {
	envAccessKey = os.Getenv("BUNNY_TEST_API_KEY")
	envZone = os.Getenv("BUNNY_TEST_ZONE")

	if len(envAccessKey) == 0 || len(envZone) == 0 {
		fmt.Println(`Please notice that this test runs agains the public Bunny.net API, so you sould
never run the test with a zone, used in production.
To run this test, you have to specify 'BUNNY_TEST_API_KEY' and 'BUNNY_TEST_ZONE'.
Example: "BUNNY_TEST_API_KEY="123" BUNNY_TEST_ZONE="my-domain.com" go test ./... -v`)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func Test_AppendRecords(t *testing.T) {
	p := &bunny.Provider{
		AccessKey: envAccessKey,
		Debug:     true,
	}

	newRecords := []libdns.Record{
		libdns.TXT{
			Name: "test_4",
			Text: "test_value_4",
			TTL:  ttl,
		},
	}

	_, cleanupFunc := setupTestRecords(t, p)
	defer cleanupFunc(newRecords)

	records, err := p.AppendRecords(context.Background(), envZone+".", newRecords)

	if err != nil {
		t.Fatal(err)
	}

	for _, newRecord := range newRecords {
		contains_ := contains(records, func(record libdns.Record) bool {
			return compareRecords(record, newRecord)
		})

		if !contains_ {
			t.Fatalf("result does not contain record %v", newRecord)
		}
	}

	records, err = p.GetRecords(context.Background(), envZone+".")

	if err != nil {
		t.Fatal(err)
	}

	for _, newRecord := range newRecords {
		contains_ := contains(records, func(record libdns.Record) bool {
			return compareRecords(record, newRecord)
		})

		if !contains_ {
			t.Fatalf("record %v does not exist in zone", newRecord)
		}
	}
}

func Test_DeleteRecords(t *testing.T) {
	p := &bunny.Provider{
		AccessKey: envAccessKey,
		Debug:     true,
	}

	_, cleanupFunc := setupTestRecords(t, p)
	defer cleanupFunc(nil)

	deletedRecords := []libdns.Record{
		libdns.TXT{
			Name: "test_3",
			Text: "test_value_3",
			TTL:  ttl,
		},
	}

	records, err := p.DeleteRecords(context.Background(), envZone, deletedRecords)

	if err != nil {
		t.Fatal(err)
	}

	for _, deletedRecord := range deletedRecords {
		contains_ := contains(records, func(record libdns.Record) bool {
			return compareRecords(record, deletedRecord)
		})

		if !contains_ {
			t.Fatalf("result does not contain record %v", deletedRecord)
		}
	}

	records, err = p.GetRecords(context.Background(), envZone)

	if err != nil {
		t.Fatal(err)
	}

	for _, deletedRecord := range deletedRecords {
		contains_ := contains(records, func(record libdns.Record) bool {
			return compareRecords(record, deletedRecord)
		})

		if contains_ {
			t.Fatalf("record %v is still present on nameserver", deletedRecord)
		}
	}
}

func Test_GetRecords(t *testing.T) {
	p := &bunny.Provider{
		AccessKey: envAccessKey,
		Debug:     true,
	}

	testRecords, cleanupFunc := setupTestRecords(t, p)
	defer cleanupFunc(nil)

	records, err := p.GetRecords(context.Background(), envZone)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) < len(testRecords) {
		t.Fatalf("len(records) < len(testRecords) => %d < %d", len(records), len(testRecords))
	}

	for _, testRecord := range testRecords {
		foundRecord := contains(records, func(record libdns.Record) bool {
			return compareRecords(record, testRecord)
		})

		if !foundRecord {
			t.Fatalf("Record not found => %v", testRecord)
		}
	}
}

func Test_SetRecords(t *testing.T) {
	p := &bunny.Provider{
		AccessKey: envAccessKey,
		Debug:     true,
	}

	updatedRecords := []libdns.Record{
		libdns.TXT{ // Update existing record
			Name: "test_3",
			Text: "test_value_3_new",
			TTL:  ttl,
		},
		libdns.TXT{ // Add new record
			Name: "test_4",
			Text: "test_value_4",
			TTL:  ttl,
		},
	}

	_, cleanupFunc := setupTestRecords(t, p)
	defer cleanupFunc(updatedRecords)

	records, err := p.SetRecords(context.Background(), envZone, updatedRecords)

	if err != nil {
		t.Fatal(err)
	}

	for _, updatedRecord := range updatedRecords {
		contains_ := contains(records, func(record libdns.Record) bool {
			return compareRecords(record, updatedRecord)
		})

		if !contains_ {
			t.Fatalf("result does not contain record %v", updatedRecord)
		}
	}

	records, err = p.GetRecords(context.Background(), envZone)

	if err != nil {
		t.Fatal(err)
	}

	for _, updatedRecord := range updatedRecords {
		contains_ := contains(records, func(record libdns.Record) bool {
			return compareRecords(record, updatedRecord)
		})

		if !contains_ {
			t.Fatalf("record %v does not exist on nameserver", updatedRecord)
		}
	}

	if len(records) != len(testRecords)+1 {
		t.Fatalf("len(records) != len(testRecords) + 1 => %d != %d", len(records), len(testRecords)+1)
	}
}

func Test_NestedRecords(t *testing.T) {
	p := &bunny.Provider{
		AccessKey: envAccessKey,
		Debug:     true,
	}

	testRecords := []libdns.Record{
		libdns.TXT{
			Name: "test1",
			Text: "test1",
			TTL:  ttl,
		},
		libdns.TXT{
			Name: "test2",
			Text: "test2",
			TTL:  ttl,
		},
	}

	_, err := p.SetRecords(context.Background(), fmt.Sprintf("subdomain.%s", envZone), testRecords)
	if err != nil {
		t.Fatal(err)
	}

	defer cleanupRecords(t, p, []libdns.Record{
		libdns.TXT{
			Name: "test1.subdomain",
			Text: "test1",
			TTL:  ttl,
		},
		libdns.TXT{
			Name: "test2.subdomain",
			Text: "test2",
			TTL:  ttl,
		},
	})

	// Check that records created on a "subdomain" are correctly suffixed.

	records, err := p.GetRecords(context.Background(), envZone)
	if err != nil {
		t.Fatal(err)
	}

	for _, testRecord := range testRecords {
		var foundRecord *libdns.Record
		for _, record := range records {
			if fmt.Sprintf("%s.subdomain", testRecord.RR().Name) == record.RR().Name {
				foundRecord = &testRecord
			}
		}

		if foundRecord == nil {
			t.Fatalf("Record not found => %s.subdomain", testRecord.RR().Name)
		}
	}

	// Check that records retrieved from a "subdomain" are normalised correctly.

	records, err = p.GetRecords(context.Background(), fmt.Sprintf("subdomain.%s", envZone))

	if err != nil {
		t.Fatal(err)
	}

	if len(records) != len(testRecords) {
		t.Fatalf("len(records) != len(testRecords) => %d != %d", len(records), len(testRecords))
	}

	for _, testRecord := range testRecords {
		var foundRecord *libdns.Record
		for _, record := range records {
			if testRecord.RR().Name == record.RR().Name {
				foundRecord = &testRecord
			}
		}

		if foundRecord == nil {
			t.Fatalf("Record not found => %s", testRecord.RR().Name)
		}
	}
}
