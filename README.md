# Bunny.net DNS for [`libdns`](https://github.com/libdns/libdns)

[![Go Reference](https://pkg.go.dev/badge/test.svg)](https://pkg.go.dev/github.com/libdns/bunny)

This package implements the [libdns interfaces](https://github.com/libdns/libdns) for [Bunny.net](https://docs.bunny.net/reference/bunnynet-api-overview), allowing you to manage DNS records.

## Authenticating

To authenticate you need to supply a Bunny.net [API Key](https://dash.bunny.net/account/settings).

## Example

Here's a minimal example of how to get all DNS records for zone. See also: [provider_test.go](https://github.com/libdns/bunny/blob/master/provider_test.go)

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/libdns/bunny"
)

func main() {
	apiKey := os.Getenv("BUNNY_API_KEY")
	if apiKey == "" {
		fmt.Printf("BUNNY_API_KEY not set\n")
		return
	}

	zone := os.Getenv("BUNNY_ZONE")
	if zone == "" {
		fmt.Printf("BUNNY_ZONE not set\n")
		return
	}

	provider := &bunny.Provider{
		AccessKey: apiKey,
	}

	records, err := provider.GetRecords(context.WithTimeout(context.Background(), time.Duration(15*time.Second)), zone)
	if err != nil {
        fmt.Printf("Error: %s", err.Error())
        return
	}

	fmt.Println(records)
}

```

## Zone management

Beyond the standard libdns interfaces, `*bunny.Provider` exposes the full Bunny.net DNS zone API: `GetZone`, `CreateZone`, `UpdateZone`, `DeleteZone`, `CheckZoneAvailability`, `ExportZone`, `ImportZone` and `GetZoneStatistics`.

```go
available, err := provider.CheckZoneAvailability(ctx, "example.com")
if err != nil || !available {
	return err
}

zone, err := provider.CreateZone(ctx, "example.com")
if err != nil {
	return err
}
fmt.Printf("created zone %d, nameservers %s / %s\n", zone.ID, zone.Nameserver1, zone.Nameserver2)

// Import records from a BIND zone file, then export them again.
result, err := provider.ImportZone(ctx, "example.com", zoneFileBytes)
zoneFile, err := provider.ExportZone(ctx, "example.com")

// Update zone settings; nil fields are left unchanged.
soa := "hostmaster@example.com"
zone, err = provider.UpdateZone(ctx, "example.com", bunny.ZoneUpdate{SoaEmail: &soa})
```

## DNSSEC

Bunny.net manages DNSSEC keys and signs zones automatically:

```go
info, err := provider.EnableDNSSEC(ctx, "example.com")
if err != nil {
	return err
}
if !info.DSConfigured {
	fmt.Printf("publish this DS record at your registrar: %s\n", info.DSRecord)
}
```

**Bunny.net does not publish the DS record at your domain's registrar.** Until you add the returned DS record at the registrar (or parent zone) yourself, DNSSEC validation is not active. Likewise, remove the DS record at the registrar *before* calling `DisableDNSSEC`, or validating resolvers will fail to resolve the zone.

## Debugging

You can enable logging by configuring a custom logger or by setting `Debug` to true.

```go
  ...

  // Logging is always enabled when using a custom logger
  provider := &bunny.Provider{
    AccessKey: apiKey,
    Logger: func(msg string, records []libdns.Record) {
      fmt.Printf("[bunny]: %s\n", msg)
    },
  }

  // Enable the default logger
  provider := &bunny.Provider{
    AccessKey: apiKey,
    Debug: true,
  }
```
Example output using the default logger:

```shell
[bunny] fetching all records in zone example.com
[bunny] fetching zone ID for example.com
[bunny] done found zone ID 82940 for example.com
[bunny] done fetching 3 record(s) in zone example.com
[bunny]   TXT: ID=7648777, TTL=2m0s, Priority=0, Name=test1, Value=test1
[bunny]   TXT: ID=7648778, TTL=2m0s, Priority=0, Name=test2, Value=test2
[bunny]   TXT: ID=7648779, TTL=2m0s, Priority=0, Name=test3, Value=test3
[bunny] deleting TXT record in zone example.com
```
