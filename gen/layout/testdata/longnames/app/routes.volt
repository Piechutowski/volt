package app

import (
	db
)

scope /api [name: api] {
	get    /rollups/:rollupIdentifier(int) db.PublicationEngagementMetricRollupsOfEditorGet
	get    /rollups                        db.PublicationEngagementMetricRollupsOfEditorLatest
	get    /engaged                        db.PublicationEngagementMetricRollupsOfEditorEngaged
	get    /opened                         db.PublicationEngagementMetricRollupsOfEditorOpened
	post   /rollups                        db.PublicationEngagementMetricRollupsOfEditorCreate
	patch  /rollups/:rollupIdentifier(int) db.PublicationEngagementMetricRollupsOfEditorUpdate
	delete /rollups/:rollupIdentifier(int) db.PublicationEngagementMetricRollupsOfEditorDelete
}

scope /dashboards [name: dashboards] {
	get /recent/:rollupIdentifier(int) Boards.PublicationOverview {
		currentEngagementRollup db.PublicationEngagementMetricRollupsOfEditorGet(rollupIdentifier)
		recentEngagementRollups db.PublicationEngagementMetricRollupsOfEditorLatest(limit)
	}
	patch /:id(int) Boards.Annotate [body: db.PublicationEngagementMetricRollupsOfEditorUpdate(id)]
}
