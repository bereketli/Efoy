// Command realtime-gateway is the WebSocket server that pushes live
// locations, ETAs, boarding and trip events to riders, guardians and ops.
package main

import "github.com/blinge12/efoy/internal/platform/bootstrap"

func main() {
	bootstrap.Run("realtime-gateway", nil)
}
