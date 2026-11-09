package main

import (
	"context"
	"fmt"
	"github.com/CarlosAmaral/rag-app/app"
	"github.com/CarlosAmaral/rag-app/config"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	defer stop()

	if err := app.Run(ctx, config.Load()); err != nil {
		fmt.Println(os.Stderr, err)
		os.Exit(1)
	}
}
