package db

pred visible { state = "published" and published_at is not null and (published_at < :at? or (published_at = :at? and id < :id?)) }

select feed (id, slug, title, published_at, author.name as author_name, count(comments.id) as comment_count) for posts join author join comments where visible and exists tags (tag = :tag?) [group: (id, slug, title, published_at, author_name), order: (published_at desc, id desc), limit: :limit]
select by_state (state, count() as n) for posts where author_id = :member [group: (state)]
select received (count(comments.id) as n) for posts join comments where author_id = :member
select of_post for post_tags where post_id = :post [order: (tag asc)]
select thread for comments where post_id = :post [order: (id asc)]
select recent (count() as n) for comments where post_id = :post and member_id = :member and created_at > :since
select trail for audits where post_id = :post [order: (id asc)]
