package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/Piechutowski/volt/nao/run"
	"github.com/Piechutowski/volt/tint"
	"shop/lib__api"
)

func main() {
	logger := slog.New(tint.NewTextHandler(os.Stderr, tint.Options{}))
	conn, err := run.Open("sqlite3", "shop.db", logger)
	if err != nil {
		panic(err)
	}
	http.ListenAndServe(":8080", api.Open(conn))
}
