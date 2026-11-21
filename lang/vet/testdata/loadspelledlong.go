// analyzers: loadspelledlong
// bodies: Save 2 db.MetricChangeset, Add 1 db.MetricChangeset, Rename 2 db.MetricChangeset
package app

import (
	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/nao/rt"
	"metrics/db"
)

type Metrics struct{}

func (Metrics) Show(r volt.Request, id int) volt.Response {
	return volt.Then(db.MetricGet(id), func(m db.Metric) volt.Response { return volt.Render(m) }) //WANT loadspelledlong
}

func (Metrics) Peers(r volt.Request, id int) volt.Response {
	unit := r.Query().Get("unit")
	return volt.Then(db.MetricByUnit(unit, 3), func(ms []db.Metric) volt.Response { return volt.Render(ms) })
}

func (Metrics) Recent(r volt.Request) volt.Response {
	return volt.Then(db.MetricQuery(rt.Limit[db.Metric](10)), func(ms []db.Metric) volt.Response { return volt.Render(ms) })
}

func (Metrics) Save(r volt.Request, id int, body db.MetricChangeset) volt.Response {
	return volt.Then(db.MetricUpdate(body), func(m db.Metric) volt.Response { return volt.Render(m) })
}

func (Metrics) Rename(r volt.Request, id int, cs db.MetricChangeset) volt.Response {
	return volt.Then(db.MetricUpdate(cs), func(m db.Metric) volt.Response { return volt.Render(m) })
}

func (Metrics) Mirror(r volt.Request, id int, body int) volt.Response {
	return volt.Then(db.MetricGet(body), func(m db.Metric) volt.Response { return volt.Render(m) }) //WANT loadspelledlong
}

func (Metrics) Drop(r volt.Request, id int) volt.Response {
	return volt.Then(db.MetricDelete(id), func(int64) volt.Response { return volt.NoContent() })
}

func (Metrics) Add(r volt.Request, body db.MetricChangeset) volt.Response {
	return volt.Then(rt.Insert(body), func(m db.Metric) volt.Response { return volt.Render(m) })
}

func (Metrics) Purge(r volt.Request) volt.Response {
	return volt.Then(db.MetricDeleteWhere(), func(int64) volt.Response { return volt.NoContent() })
}

func (Metrics) Touched(r volt.Request, id int) volt.Response {
	return volt.Then(db.MetricLastUpdate(id), func(m db.Metric) volt.Response { return volt.Render(m) }) //WANT loadspelledlong
}
