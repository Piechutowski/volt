// spec: §tablepartial, two indexes on one key inside one partial are that partial's own redundancy, as in a table body, not an index injected twice
TablePartial stamps {
  created_at time.Time
  indexes {
    created_at
    created_at [name: 'idx_posts_created']
  }
}
Table posts {
  id int [pk]
  ~stamps
}
