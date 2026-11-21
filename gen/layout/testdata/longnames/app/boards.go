package app

import (
	"github.com/Piechutowski/volt"
	"longnames/db"
)

type Boards struct{}

func (Boards) PublicationOverview(
	r volt.Request,
	rollupIdentifier int64,
	currentEngagementRollup db.PublicationEngagementMetricRollupsOfEditor,
	recentEngagementRollups []db.PublicationEngagementMetricRollupsOfEditor,
) volt.Response {
	return volt.Render(recentEngagementRollups)
}

func (Boards) Annotate(
	r volt.Request,
	rollupIdentifier int64,
	body db.PublicationEngagementMetricRollupsOfEditorChangeset,
) volt.Response {
	return volt.NoContent()
}
