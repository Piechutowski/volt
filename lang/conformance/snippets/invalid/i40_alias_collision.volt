// spec: §namespaces-and-uniqueness — aliases share the table-name namespace
Table users { id int }
Table people as users { id int }
