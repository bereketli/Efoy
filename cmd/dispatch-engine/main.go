// Command dispatch-engine runs incident detection on the location stream,
// ranks replacement candidates and sends replacement offers.
package main

import "github.com/blinge12/efoy/internal/platform/bootstrap"

func main() {
	bootstrap.Run("dispatch-engine", nil)
}
