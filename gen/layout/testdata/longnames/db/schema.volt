package db

table publication_engagement_metric_rollups_of_editors {
	rollup_identifier                 int       [pk]
	publication_reference_identifier  int
	cumulative_reader_session_seconds int       [default: 0]
	editorial_annotation_text         string    [null]
	aggregation_window_started_at     time.Time
}

select latest for publication_engagement_metric_rollups_of_editors [order: (rollup_identifier desc), limit: :limit]
select engaged for publication_engagement_metric_rollups_of_editors where cumulative_reader_session_seconds >= :seconds?
select opened for publication_engagement_metric_rollups_of_editors where aggregation_window_started_at > :after
