// spec: §tablepartial, two indexes on one key inside one partial are that partial's own redundancy, as in a table body, not an index injected twice
TablePartial stamps {
  created_at time.Time
  updated_at time.Time
  Indexes {
    (created_at, updated_at)
    (created_at, updated_at) [name: "idx_posts_stamps"]
  }
}
Table posts {
  id int [pk]
  ~stamps
}
