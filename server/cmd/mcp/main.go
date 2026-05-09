package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

func main() {
	ctx := context.Background()
	logger := slog.Default()

	if err := run(ctx, logger); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	fmt.Println("agentra-mcp starting...")
	return nil
}
