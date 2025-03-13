package pluginutils

import (
	"os"
	"os/signal"
	"syscall"
)

// WaitForShutdown is a helper function that waits for a shutdown signal
// and then calls the provided closer function.
func WaitForShutdown(closer func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(
		c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM,
	)

	<-c
	closer()
}
