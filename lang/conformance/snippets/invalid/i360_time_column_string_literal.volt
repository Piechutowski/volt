// spec: §predicates, a time.Time column takes no literal: there is no date/time literal, and a string is text
// want: column "at" (time.Time) takes no literal
package app

table events {
	id int       [pk]
	at time.Time
}

select since for events where at > "2024-01-01"
