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
	if token == "" {
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

## Debugging

You can enable logging by configuring an external logger or by setting `Debug` to true.

```go
	...

	// Use an external logger
	provider := &bunny.Provider{
		AccessKey: apiKey,
		Logger: func(msg string) {
			fmt.Printf("Info: %s\n", msg)
		}
	}

	// Use the internal logger
	provider := &bunny.Provider{
		AccessKey: apiKey,
		Debug: true
	}
```
Example output using the internal logger:

```shell
[bunny] getting zone ID for example.com
[bunny] got zone ID 82940 for example.com
[bunny] deleting record 7640618 in zone 82940: Name=test3, Value=test3
```
