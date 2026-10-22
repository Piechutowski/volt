; Volt notes conventionally contain Markdown (docs/spec.md §notes): inject Zed's
; built-in Markdown language into every note: setting, on a project, a table
; or a column.
(setting
  name: (setting_name) @_key
  value: (string
    content: (string_content) @injection.content)
  (#eq? @_key "note")
  (#set! injection.language "markdown"))

; backtick expressions are opaque SQL — highlights if a SQL extension is
; installed, harmless otherwise
(expression
  content: (expression_content) @injection.content
  (#set! injection.language "sql"))
