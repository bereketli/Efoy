// Command tracking-ingest receives GPS batches from driver apps, writes the latest
// position to Redis GEO, appends history to TimescaleDB and publishes
// location events to NATS.
package main

import "github.com/blinge12/efoy/internal/platform/bootstrap"

func main() {
	bootstrap.Run("tracking-ingest", nil)
}
