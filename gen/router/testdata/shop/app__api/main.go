package main

import (
	"net/http"

	"github.com/Piechutowski/volt/nao/run"
	"shop/lib__api"
)

func main() {
	conn, err := run.Open("sqlite3", "shop.db")
	if err != nil {
		panic(err)
	}
	http.ListenAndServe(":8080", api.Open(conn))
}
