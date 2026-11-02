// spec: §request-bodies, a controller route binds its body to a changeset of a write, narrowed by only:, or to a struct of the package
package app

import (
	db
)

Scope / {
	patch /settings/:id(int) Settings.Save   [body: db.SettingUpdate(id), only: (refresh, note)]
	patch /settings/first    Settings.First  [body: db.SettingUpdate(1)]
	post  /metrics           Metrics.Create  [body: db.MetricCreate]
	post  /metrics/batch     Metrics.Batch   [body: db.MetricCreate()] {
		all db.MetricOrdered()
	}
	post  /login             Session.Login   [body: LoginForm]
}
