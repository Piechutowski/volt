// spec: §keywords, §selects-over-groups, §predicates, §types, clause words are lowercase keywords: FOR, WHERE, AND, IS, NULL and FROM are errors naming the right spelling
// want: write where, not WHERE
package db

Type money.Money string FROM "example.com/money"

Table posts {
  id    int    [pk]
  title string [null]
  views int
}

Select fresh for posts WHERE views > 0 AND title IS NOT NULL
Select all FOR posts
