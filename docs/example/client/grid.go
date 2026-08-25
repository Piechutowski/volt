// YOURS — the guigui desktop client. It imports the SAME generated db
// package (nao --models-only) and decodes gob into the same structs:
// schema → structs → SQL → wire → widget, one type the whole way.
package client

import (
	"encoding/gob"
	"net/http"

	"github.com/Piechutowski/volt/dataset"

	"example.com/fadn/db"
)

func FetchDaRR(base, token string, year int64) (dataset.Page[db.DaRR], error) {
	var page dataset.Page[db.DaRR]

	req, _ := http.NewRequest("GET", base+"/da/r_r?f.rok="+itoa(year)+"&sort=-idpk", nil)
	req.Header.Set("Authorization", token)
	req.Header.Set("Accept", "application/x-gob") // the Go-native arm

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return page, err
	}
	defer resp.Body.Close()

	// page.Rows is []db.DaRR — no DTOs, no mapping, no JSON tags.
	// page.Columns carries Title/Unit/Dict for headers, zł/ha formatting,
	// and słownik dropdowns in the grid widget.
	err = gob.NewDecoder(resp.Body).Decode(&page)
	return page, err
}
