// YOURS — the guigui desktop client. It imports the SAME generated db
// package (nao --models-only) and decodes gob into the same structs:
// schema → structs → SQL → wire → widget, one type the whole way.
package client

import (
	"encoding/gob"
	"net/http"

	"github.com/Piechutowski/volt/dataset"

	"example.com/metrics/db"
)

func FetchMsRevenue(base, token string, year int64) (dataset.Page[db.MsRevenue], error) {
	var page dataset.Page[db.MsRevenue]

	req, _ := http.NewRequest("GET", base+"/ms/revenue?f.year="+itoa(year)+"&sort=-id", nil)
	req.Header.Set("Authorization", token)
	req.Header.Set("Accept", "application/x-gob") // the Go-native arm

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return page, err
	}
	defer resp.Body.Close()

	// page.Rows is []db.MsRevenue — no DTOs, no mapping, no JSON tags.
	// page.Columns carries Title/Unit/Dict for headers, EUR/%/seats
	// formatting, and dictionary dropdowns in the grid widget.
	err = gob.NewDecoder(resp.Body).Decode(&page)
	return page, err
}
