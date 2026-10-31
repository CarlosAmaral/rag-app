package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"rag-app/app"
	"rag-app/config"
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
