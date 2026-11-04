// spec: §predicates, an optional parameter :name? is rt.Null of its type, optional at every use, its term true when it is absent, a list one included, a keyset over a nullable column keeping its NULLs out
package db

table posts {
  id           int       [pk]
  title        string
  state        string
  published_at time.Time [null]
}

pred before { published_at < :at? or (published_at = :at? and id < :id?) }

select feed for posts where published_at is not null and before and state = "published" [order: (published_at desc, id desc), limit: :limit]
select chosen for posts where id in :ids? and title like :pattern? [order: (id asc)]
select since for posts where :from? <= published_at and not state = "draft"
