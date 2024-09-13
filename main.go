package main

import (
	"context"
	"log"
	"os"
)

func main() {
	app := &application{
		dev: os.Getenv("BRANCH") == "dev",
	}

	ctx := context.Background()
	if err := app.run(ctx, "80", "443"); err != nil {
		log.Fatal(err.Error())
	}
}
