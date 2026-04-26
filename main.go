package main

import (
	"fmt"
	"os"

	"github.com/skrptiq/skrptiq-cli/internal/app"
)

func main() {
	a, err := app.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer a.Close()
	a.Run()
}
