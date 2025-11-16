package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	ig2wacmd "ig2wa/internal/cli/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := ig2wacmd.Execute(ctx); err != nil {
		var ee *ig2wacmd.ExitError
		if errors.As(err, &ee) {
			if ee.Err != nil {
				fmt.Fprintln(os.Stderr, ee.Err)
			}
			os.Exit(ee.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(ig2wacmd.ExitCLIError)
	}
	os.Exit(ig2wacmd.ExitOK)
}
