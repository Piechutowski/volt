; Volt highlight queries for Zed. Later patterns win, so generic rules come
; first and specific roles override them.

; ---------- comments and literals ----------

(comment) @comment

(string) @string
(number) @number
(signed_number) @number
(boolean) @boolean
(null) @constant.builtin
(color) @constant

; opaque SQL expressions in backticks
(expression) @string.special

; ---------- keywords ----------

[
  "Project"
  "Table"
  "TablePartial"
  "Enum"
  "Ref"
  "TableGroup"
  "Note"
  "Records"
  "DiagramView"
  "indexes"
  "checks"
  "as"
  "use"
  "reuse"
  "from"
  "Tables"
  "Notes"
  "TableGroups"
  "Schemas"
] @keyword

(element_kind) @keyword

; ---------- operators and punctuation ----------

(cardinality) @operator
"~" @operator
(wildcard) @operator

["{" "}" "(" ")" "[" "]"] @punctuation.bracket
[":" "," "."] @punctuation.delimiter

; ---------- names by role ----------

; schema qualifiers: core.users, v2.job_status
(schema_name) @type @namespace

; tables (definitions, aliases and references)
(table_name name: (_) @type)
(table_alias) @type
(group_member) @type
(view_member) @type
(ref_target (_) @type)

; enums
(enum_definition name: (table_name name: (_) @enum))
(enum_value name: (_) @variant)
(enum_constant enum: (_) @enum value: (_) @variant)

; table partials
(partial_name) @constructor

; columns
(column_name) @property
(column_definition name: (_) @property)
(records_columns (column_name) @property)

; column types
(type_name) @type

; project
(project_definition name: (_) @title)
(project_property key: (_) @property)

; named refs, sticky notes, groups, views
(ref_name) @label
(sticky_note_definition name: (_) @label)
(table_group_definition name: (_) @label)
(diagram_view_definition name: (_) @label)

; ---------- settings ----------

(settings_list (setting name: (_) @attribute))
(setting_value_words) @constant
(legacy_flag) @attribute

; ---------- Volt layer (docs/spec.md §V) ----------

[
  "package"
  "import"
  "Pipeline"
  "Scope"
  "resources"
] @keyword

(verb) @keyword

; module wiring
(package_clause name: (identifier) @namespace)
(import_alias) @namespace
(import_path (identifier) @namespace)

; pipelines and Go references (plugs, route handlers, error handlers)
(pipeline_name) @label
(go_ref (identifier) @function)

; route paths: literals as paths, parameters as parameters
(route_path "/" @string.special)
(path_segment) @string.special
(path_parameter ":" @operator)
(parameter_name) @variable.parameter
(wildcard_marker) @operator

; the resources table reference
(resources_declaration table: (table_name) @type)

; ---------- groups, predicates, selects (docs/spec.md §V9-§V11) ----------

[
  "Group"
  "Pred"
  "Select"
  "for"
  "where"
] @keyword

[
  "and"
  "or"
  "not"
  "in"
  "like"
  "is"
  "null"
] @keyword.operator

(group_name) @type
(group_member) @type
(group_definition ["+" "\\"] @operator)
(pred_name) @label
(pred_ref) @label
(select_name) @label
(select_target) @type
(column_ref) @property
(query_param ":" @operator)
(query_param name: (parameter_name) @variable.parameter)
(pred_compare op: _ @operator)
(ident_modifier) @constant
