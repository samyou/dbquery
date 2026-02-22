package main

import (
	"fmt"
	"os"

	"dbquery/internal/dbquery"
)

func main() {
	if err := dbquery.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
