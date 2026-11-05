; backtick expressions are opaque SQL — highlights if a SQL extension is
; installed, harmless otherwise
(expression
  content: (expression_content) @injection.content
  (#set! injection.language "sql"))
