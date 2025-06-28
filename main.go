package main

import (
	"context"
	"github.com/phuslu/lru"
	"log"
	"os"
)

var cache *lru.LRUCache[string, string]

func init() {
	cache = lru.NewLRUCache[string, string](1024)
}

func main() {
	app := &application{
		dev: os.Getenv("BRANCH") == "dev",
	}

	ctx := context.Background()
	if err := app.run(ctx, "443"); err != nil {
		log.Fatal(err.Error())
	}
}
