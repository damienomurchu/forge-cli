package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/damienomurchu/forge-cli/spikes/sqlite"
)

func main() {
	databasePath := flag.String("db", "", "path to a temporary SQLite database")
	migrate := flag.Bool("migrate", false, "apply the initial migration before checking the schema")
	flag.Parse()
	if *databasePath == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: sqlite-spike -db PATH [-migrate]")
		os.Exit(2)
	}

	db, err := sqliteprobe.Open(*databasePath)
	if err == nil {
		defer db.Close()
		if *migrate {
			err = sqliteprobe.Migrate(context.Background(), db)
		} else {
			err = db.PingContext(context.Background())
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
