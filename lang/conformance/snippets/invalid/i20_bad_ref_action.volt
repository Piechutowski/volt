// spec: §relationships-ref, a referential action is one of the four listed
table a { x int }
table b { id int }
ref: a.x > b.id [delete: explode]
