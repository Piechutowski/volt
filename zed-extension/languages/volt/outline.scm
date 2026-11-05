(table_definition
  "table" @context
  name: (table_name) @name) @item

(table_partial_definition
  "table_partial" @context
  name: (partial_name) @name) @item

(type_definition
  "type" @context
  name: (type_name) @name) @item

(column_definition
  name: (column_name) @name
  type: (column_type) @context.extra) @item

(indexes_block
  "indexes" @name) @item

(validators_block
  "validators" @name) @item

(ref_definition
  "ref" @context
  (ref_relation) @name) @item

(records_definition
  "records" @context
  table: (table_name) @name) @item

; ---------- Volt layer (docs/spec.md Part II) ----------

(package_clause
  "package" @context
  name: (identifier) @name) @item

(pipeline_definition
  "pipeline" @context
  name: (pipeline_name) @name) @item

(scope_definition
  "scope" @context
  path: (route_path) @name) @item

(route
  verb: (verb) @context
  path: (route_path) @name) @item

(resources_declaration
  "resources" @context
  table: (table_name) @name) @item

(group_definition
  "group" @context
  name: (group_name) @name) @item

(pred_definition
  "pred" @context
  name: (pred_name) @name) @item

(select_definition
  "select" @context
  name: (select_name) @name) @item

(migration_definition
  "migration" @context
  name: (migration_name) @name) @item
