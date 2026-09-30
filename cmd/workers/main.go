// Command workers runs River background jobs (trip generation, billing,
// payouts, reminders, notifications) and the transactional outbox relay.
package main

import "github.com/blinge12/efoy/internal/platform/bootstrap"

func main() {
	bootstrap.Run("workers", nil)
}
