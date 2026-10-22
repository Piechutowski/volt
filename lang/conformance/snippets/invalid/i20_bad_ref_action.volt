// spec: §relationships-ref, a referential action is one of the four listed
Table a { x int }
Table b { id int }
Ref: a.x > b.id [delete: explode]
