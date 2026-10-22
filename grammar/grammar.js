/**
 * Tree-sitter grammar for the Volt language (.volt files): the schema core,
 * the schema extensions, and the routing layer above them.
 *
 * Covers the full schema language of docs/spec.md Part I:
 * Project, Table (settings, columns), TablePartial and ~injection,
 * Type, Ref (the element and the inline ref: setting, composite
 * endpoints), Records (top-level and in-table), Indexes and Validators
 * blocks, plus the Volt routing layer of docs/spec.md Part II: package clauses, Go-style
 * import blocks, Pipeline, Scope, routes and resources. One grammar for
 * one language in three layers (§layers-files-and-the-superset-rule): every .volt file parses here,
 * whichever layer it uses.
 *
 * Design notes, driven by spec §lexical-structure:
 *  - Volt is newline-sensitive: columns, record rows, index and
 *    validator lines, project properties, group members and partial injections
 *    are newline-terminated. Newlines are therefore NOT extras; bodies use an
 *    explicit "items separated by one-or-more newlines" shape, which also
 *    accepts a final item flush against the closing brace.
 *  - Keywords are case-sensitive, have one spelling each, and are NOT
 *    reserved (§keywords). Each keyword is a plain string, which
 *    tree-sitter's keyword extraction matches against the identifier
 *    token exactly, only where the parse state admits it. Consequence
 *    (documented limitation): a column named exactly `Indexes`,
 *    `Validators` or `Records` lexes as the block keyword at the start
 *    of a table-body line.
 *  - A name is a Go identifier and is never quoted (§identifiers). Double
 *    quotes delimit strings, as in Go, and triple double quotes a
 *    multi-line string (§string-literals). String and expression contents
 *    are separate tokens so editors can inject other languages (Markdown
 *    in notes, SQL in backticks).
 *  - Settings lists are generic `[name, key: value]` (§settings-lists); which keys are
 *    valid where is the language server's business, which also keeps the
 *    grammar open for the schema extensions.
 */

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

/** A keyword in its one spelling (§keywords). */
function kw(word) {
  return word;
}

/** One-or-more `rule`s separated by one-or-more newlines, then optional
 * trailing newlines. Callers wrap in optional() for possibly-empty bodies. */
function newlineSep1(rule, $) {
  return seq(rule, repeat(seq(repeat1($._newline), rule)), repeat($._newline));
}

module.exports = grammar({
  name: 'volt',

  // Newlines are significant; \r is discarded everywhere (§source-text). Comments may
  // appear anywhere and a line comment does not consume its newline (§comments).
  extras: ($) => [/[ \t\r]/, $.comment],

  word: ($) => $.identifier,

  rules: {
    source_file: ($) =>
      seq(
        repeat($._newline),
        optional(newlineSep1($._element, $)),
      ),

    _element: ($) =>
      choice(
        $.project_definition,
        $.table_definition,
        $.table_partial_definition,
        $.type_definition,
        $.ref_definition,
        $.records_definition,
        // Volt layer (docs/spec.md Part II)
        $.package_clause,
        $.import_block,
        $.pipeline_definition,
        $.scope_definition,
        $.group_definition,
        $.pred_definition,
        $.select_definition,
        $.migration_definition,
      ),

    // ==================== Project (§project) ====================

    project_definition: ($) =>
      seq(
        kw('Project'),
        optional(field('name', $._name)),
        optional(field('settings', $.settings_list)),
        '{',
        repeat($._newline),
        optional(newlineSep1($.project_property, $)),
        '}',
      ),

    project_property: ($) =>
      seq(field('key', $._name), ':', field('value', $.string)),

    // ==================== Table (§table, §columns) ====================

    table_definition: ($) =>
      seq(
        kw('Table'),
        field('name', $.table_name),
        optional(field('settings', $.settings_list)),
        '{',
        repeat($._newline),
        optional(newlineSep1($._table_item, $)),
        '}',
      ),

    _table_item: ($) =>
      choice(
        $.column_definition,
        $.indexes_block,
        $.validators_block,
        $.partial_injection,
        $.records_block,
      ),

    // A table name: a declaration, a records target or a ref endpoint's table.
    table_name: ($) => field('name', $._name),

    column_definition: ($) =>
      seq(
        field('name', alias($._name, $.column_name)),
        field('type', $.column_type),
        optional(field('settings', $.settings_list)),
      ),

    column_type: ($) =>
      seq(
        optional(seq('[', ']')),
        field('name', $.type_name),
      ),

    // A Go type name, optionally qualified by a package: time.Time, money.Money.
    type_name: ($) =>
      seq(
        optional(seq(field('package', alias($._name, $.package_name)), '.')),
        field('name', $._name),
      ),

    // ==================== TablePartial (§tablepartial) ====================

    table_partial_definition: ($) =>
      seq(
        kw('TablePartial'),
        field('name', alias($._name, $.partial_name)),
        '{',
        repeat($._newline),
        optional(newlineSep1($._partial_item, $)),
        '}',
      ),

    _partial_item: ($) =>
      choice($.column_definition, $.indexes_block, $.validators_block),

    partial_injection: ($) =>
      seq('~', field('name', alias($._name, $.partial_name))),

    // ==================== Type (§types) ====================

    type_definition: ($) =>
      seq(
        kw('Type'),
        field('name', $.type_name),
        field('storage', $.column_type),
        optional(seq(kw('from'), field('path', $.string))),
      ),

    // ==================== Ref (§relationships-ref) ====================

    // Ref: posts.user_id > users.id [settings]; the foreign key is left of '>'.
    ref_definition: ($) =>
      seq(
        kw('Ref'),
        ':',
        $.ref_relation,
        optional(field('settings', $.settings_list)),
      ),

    ref_relation: ($) =>
      seq(
        field('left', $.ref_endpoint),
        '>',
        field('right', $.ref_endpoint),
      ),

    // table.column, or table.(col, col) for a composite key.
    ref_endpoint: ($) =>
      seq(
        field('table', $.table_name),
        '.',
        field('columns', choice(alias($._name, $.column_name), $.column_group)),
      ),

    column_group: ($) =>
      seq(
        '(',
        alias($._name, $.column_name),
        repeat(seq(',', alias($._name, $.column_name))),
        ')',
      ),

    // ==================== Indexes / Validators (§indexes, §validators) ====================

    indexes_block: ($) =>
      seq(
        kw('Indexes'),
        '{',
        repeat($._newline),
        optional(newlineSep1($.index_definition, $)),
        '}',
      ),

    index_definition: ($) =>
      seq(
        choice($._index_atom, $.composite_index),
        optional(field('settings', $.settings_list)),
      ),

    _index_atom: ($) => choice(alias($._name, $.column_name), $.expression),

    composite_index: ($) =>
      seq('(', $._index_atom, repeat(seq(',', $._index_atom)), ')'),

    validators_block: ($) =>
      seq(
        kw('Validators'),
        '{',
        repeat($._newline),
        optional(newlineSep1($.validator, $)),
        '}',
      ),

    // One validator line (§validators): a Go function of the package over columns.
    validator: ($) =>
      seq(
        field('ref', $.go_ref),
        '(',
        alias($.identifier, $.column_ref),
        repeat(seq(',', alias($.identifier, $.column_ref))),
        ')',
      ),


    // ==================== Records (§records-sample-data) ====================

    // Top level: Records users(id, name) { ... }
    records_definition: ($) =>
      seq(
        kw('Records'),
        field('table', $.table_name),
        field('columns', $.records_columns),
        $._records_body,
      ),

    // In-table: Records { ... } or Records (id, name) { ... }
    records_block: ($) =>
      seq(
        kw('Records'),
        optional(field('columns', $.records_columns)),
        $._records_body,
      ),

    records_columns: ($) =>
      seq(
        '(',
        alias($._name, $.column_name),
        repeat(seq(',', alias($._name, $.column_name))),
        ')',
      ),

    _records_body: ($) =>
      seq(
        '{',
        repeat($._newline),
        optional(newlineSep1($.record_row, $)),
        '}',
      ),

    // A row is comma-separated values, none of them empty (§records-sample-data).
    record_row: ($) =>
      seq($._record_value, repeat(seq(',', $._record_value))),

    _record_value: ($) =>
      choice(
        $.string,
        $.signed_number,
        $.number,
        $.boolean,
        $.null,
        $.expression,
      ),


    // ==================== Volt layer (docs/spec.md Part II) ====================
    // The routing dialect: package clauses, Go-style import blocks,
    // Pipeline and Scope. Route paths are lexical islands (§route-paths) —
    // token.immediate stitches their segments together so interior
    // whitespace ends the path, exactly like the reference parser.

    // package name (§projects-and-packages)
    package_clause: ($) =>
      seq(kw('package'), field('name', $.identifier)),

    // import ( [alias] path/segments ... ) (§imports)
    import_block: ($) =>
      seq(
        kw('import'),
        '(',
        repeat($._newline),
        optional(newlineSep1($.import_spec, $)),
        ')',
      ),

    import_spec: ($) =>
      choice(
        field('path', $.import_path),
        seq(
          field('alias', alias($.identifier, $.import_alias)),
          field('path', $.import_path),
        ),
      ),

    import_path: ($) =>
      seq(
        $.identifier,
        repeat(seq(token.immediate('/'), alias($._ident_immediate, $.identifier))),
      ),

    // Pipeline name { use ref ... } (§pipelines)
    pipeline_definition: ($) =>
      seq(
        kw('Pipeline'),
        field('name', alias($.identifier, $.pipeline_name)),
        '{',
        repeat($._newline),
        optional(newlineSep1($.plug, $)),
        '}',
      ),

    plug: ($) => seq(kw('use'), field('ref', $.go_ref)),

    go_ref: ($) => seq($.identifier, optional(seq('.', $.identifier))),

    // Scope path [settings] { routes | resources | Scope } (§scopes-and-routes)
    scope_definition: ($) =>
      seq(
        kw('Scope'),
        field('path', $.route_path),
        optional(field('settings', $.settings_list)),
        '{',
        repeat($._newline),
        optional(newlineSep1($._scope_item, $)),
        '}',
      ),

    _scope_item: ($) =>
      choice($.route, $.resources_declaration, $.dataset_declaration, $.scope_definition),

    route: ($) =>
      seq(
        field('verb', $.verb),
        field('path', $.route_path),
        field('handler', $.go_ref),
        optional(field('settings', $.settings_list)),
      ),

    verb: () =>
      choice(
        kw('get'), kw('post'), kw('put'), kw('patch'),
        kw('delete'), kw('options'), kw('head'), kw('any'),
      ),

    // resources names a table, optionally qualified by a package
    // (volt §declaration): `resources db.posts`.
    resources_declaration: ($) =>
      seq(
        kw('resources'),
        optional(seq(field('package', alias($.identifier, $.package_name)), '.')),
        field('table', alias($.identifier, $.table_name)),
        optional(field('settings', $.settings_list)),
      ),

    // dataset names a group select of an imported data package (§datasets):
    // `dataset db.browse [strip: "da_"]`, one query route per member.
    dataset_declaration: ($) =>
      seq(
        kw('dataset'),
        optional(seq(field('package', alias($.identifier, $.package_name)), '.')),
        field('select', alias($.identifier, $.select_name)),
        optional(field('settings', $.settings_list)),
      ),

    // ==================== Groups, Preds, Selects (§groups to §selects-over-groups) ====================

    // Group name { members } | Group name = a + b \ c (§groups)
    group_definition: ($) =>
      seq(
        kw('Group'),
        field('name', alias($.identifier, $.group_name)),
        choice(
          seq(
            '{',
            repeat($._newline),
            optional(newlineSep1(alias($.identifier, $.group_member), $)),
            '}',
          ),
          seq(
            '=',
            $._group_term,
            repeat(seq(choice('+', '\\'), $._group_term)),
          ),
        ),
      ),

    // One group term: a name, or a parenthesized set of names (§groups).
    _group_term: ($) =>
      choice(
        alias($.identifier, $.group_member),
        seq(
          '(',
          alias($.identifier, $.group_member),
          repeat(seq(',', alias($.identifier, $.group_member))),
          ')',
        ),
      ),

    // Pred name { expr } (§predicates) — the closed predicate language.
    pred_definition: ($) =>
      seq(
        kw('Pred'),
        field('name', alias($.identifier, $.pred_name)),
        '{',
        repeat($._newline),
        field('body', $.pred_expr),
        repeat($._newline),
        '}',
      ),

    pred_expr: ($) => $._pred_or,

    _pred_or: ($) =>
      prec.left(1, choice(
        seq($._pred_or, kw('or'), $._pred_and),
        $._pred_and,
      )),

    _pred_and: ($) =>
      prec.left(2, choice(
        seq($._pred_and, kw('and'), $._pred_unary),
        $._pred_unary,
      )),

    _pred_unary: ($) =>
      choice(seq(kw('not'), $._pred_unary), $._pred_primary),

    _pred_primary: ($) =>
      choice(
        seq('(', $.pred_expr, ')'),
        $.pred_compare,
        $.pred_in,
        $.pred_like,
        $.pred_null,
        alias($.identifier, $.pred_ref),
      ),

    pred_compare: ($) =>
      seq(
        $._pred_operand,
        field('op', choice('=', '!=', '<', '<=', '>', '>=')),
        $._pred_operand,
      ),

    pred_in: ($) =>
      seq(
        alias($.identifier, $.column_ref),
        kw('in'),
        choice(
          seq('(', $._pred_literal, repeat(seq(',', $._pred_literal)), ')'),
          $.query_param,
        ),
      ),

    pred_like: ($) =>
      seq(
        alias($.identifier, $.column_ref),
        kw('like'),
        choice($.string, $.query_param),
      ),

    pred_null: ($) =>
      seq(
        alias($.identifier, $.column_ref),
        kw('is'),
        optional(kw('not')),
        kw('null'),
      ),

    _pred_operand: ($) =>
      choice(
        $.query_param,
        $._pred_literal,
        alias($.identifier, $.column_ref),
      ),

    _pred_literal: ($) =>
      choice($.number, seq('-', $.number), $.string, $.boolean),

    query_param: ($) =>
      seq(':', field('name', alias($._ident_immediate, $.parameter_name))),

    // Select name [(cols) | (* \ cols)] for target [where expr]
    // [settings] (§selects-over-groups, §predicates)
    select_definition: ($) =>
      seq(
        kw('Select'),
        field('name', alias($.identifier, $.select_name)),
        optional(field('projection', $.select_projection)),
        kw('for'),
        field('target', alias($.identifier, $.select_target)),
        optional(seq(kw('where'), field('where', $.pred_expr))),
        optional(field('settings', $.settings_list)),
      ),

    // (a, b, c) or (* \ a \ b) (§selects-over-groups): clauses read in SQL order
    select_projection: ($) =>
      seq(
        '(',
        choice(
          seq(
            alias($.identifier, $.column_ref),
            repeat(seq(',', alias($.identifier, $.column_ref))),
          ),
          seq(
            '*',
            repeat1(
              seq(
                '\\',
                choice(
                  alias($.identifier, $.column_ref),
                  seq(
                    '(',
                    alias($.identifier, $.column_ref),
                    repeat(seq(',', alias($.identifier, $.column_ref))),
                    ')',
                  ),
                ),
              ),
            ),
          ),
        ),
        ')',
      ),

    // /users/:id(int)/avatar, /files/:path..., or the bare root /
    // (§route-paths). Segments abut: whitespace ends the path.
    route_path: ($) =>
      seq(
        '/',
        optional(
          seq(
            $._path_segment,
            repeat(seq(token.immediate('/'), $._path_segment)),
          ),
        ),
      ),

    _path_segment: ($) =>
      choice(
        alias($._ident_immediate, $.path_segment),
        $.path_parameter,
      ),

    path_parameter: ($) =>
      seq(
        token.immediate(':'),
        field('name', alias($._ident_immediate, $.parameter_name)),
        optional(
          choice(
            seq(
              token.immediate('('),
              field('type', alias($._ident_immediate, $.type_name)),
              token.immediate(')'),
            ),
            field('wildcard', alias(token.immediate('...'), $.wildcard_marker)),
          ),
        ),
      ),

    // An identifier glued to the previous token (route and import paths).
    _ident_immediate: () =>
      token.immediate(/[\p{L}_][\p{L}\p{Nd}_]*/),

    // ==================== Migration (§migrations) ====================
    // The words of a migration body are keyword tokens here and nowhere
    // else, so the context-aware lexer keeps `up` or `table` a column
    // name in a Table body (§keywords).

    migration_definition: ($) =>
      seq(
        kw('Migration'),
        field('name', $.migration_name),
        optional(field('settings', $.settings_list)),
        '{',
        repeat($._newline),
        optional(
          choice(
            newlineSep1($._operation, $),
            seq(
              $.up_block,
              repeat($._newline),
              optional(seq($.down_block, repeat($._newline))),
            ),
          ),
        ),
        '}',
      ),

    up_block: ($) => seq(kw('up'), $._operations_body),

    down_block: ($) => seq(kw('down'), $._operations_body),

    _operations_body: ($) =>
      seq('{', repeat($._newline), optional(newlineSep1($._operation, $)), '}'),

    _operation: ($) =>
      choice(
        $.create_table_op,
        $.drop_table_op,
        $.rename_table_op,
        $.alter_table_op,
        $.rebuild_table_op,
        $.create_index_op,
        $.drop_index_op,
        $.exec_op,
      ),

    create_table_op: ($) =>
      seq(kw('create'), kw('table'), field('table', alias($._name, $.table_name)), $._shape_body),

    drop_table_op: ($) =>
      seq(kw('drop'), kw('table'), field('table', alias($._name, $.table_name)), optional($._shape_body)),

    rename_table_op: ($) =>
      seq(
        kw('rename'),
        kw('table'),
        field('from', alias($._name, $.table_name)),
        kw('to'),
        field('to', alias($._name, $.table_name)),
      ),

    alter_table_op: ($) =>
      seq(
        kw('alter'),
        kw('table'),
        field('table', alias($._name, $.table_name)),
        '{',
        repeat($._newline),
        optional(newlineSep1($._alter_item, $)),
        '}',
      ),

    _alter_item: ($) => choice($.add_column, $.drop_column, $.rename_column),

    add_column: ($) => seq(kw('add'), $.column_definition),

    // The type and settings are the dropped column's shape, written for
    // reversibility (§migrations).
    drop_column: ($) =>
      seq(
        kw('drop'),
        field('name', alias($._name, $.column_name)),
        optional(seq(field('type', $.column_type), optional(field('settings', $.settings_list)))),
      ),

    rename_column: ($) =>
      seq(
        kw('rename'),
        field('from', alias($._name, $.column_name)),
        kw('to'),
        field('to', alias($._name, $.column_name)),
      ),

    rebuild_table_op: ($) =>
      seq(kw('rebuild'), kw('table'), field('table', alias($._name, $.table_name)), $._shape_body),

    // Columns and Indexes blocks only: no injection, Validators or Records
    // (§migrations).
    _shape_body: ($) =>
      seq(
        '{',
        repeat($._newline),
        optional(newlineSep1(choice($.column_definition, $.indexes_block), $)),
        '}',
      ),

    create_index_op: ($) =>
      seq(kw('create'), kw('index'), field('table', alias($._name, $.table_name)), $.index_definition),

    drop_index_op: ($) =>
      seq(kw('drop'), kw('index'), field('table', alias($._name, $.table_name)), $.index_definition),

    // exec takes its SQL in backticks, an expression literal (§migrations).
    exec_op: ($) =>
      seq(kw('exec'), field('sql', $.expression), optional(field('settings', $.settings_list))),

    // ==================== Settings (§settings-lists) ====================

    // A line break inside the brackets is whitespace (§line-structure-and-whitespace).
    settings_list: ($) =>
      seq(
        '[',
        optional($._line_breaks),
        $.setting,
        repeat(seq(optional($._line_breaks), ',', optional($._line_breaks), $.setting)),
        optional($._line_breaks),
        ']',
      ),

    _line_breaks: ($) => repeat1($._newline),

    // `ref:` takes an endpoint, table.column (§relationships-ref); every other
    // setting takes a value. A line break may follow the `:`
    // (§line-structure-and-whitespace).
    setting: ($) =>
      choice(
        seq(
          field('name', alias(kw('ref'), $.setting_name)),
          optional(seq(':', optional($._line_breaks), field('value', $.ref_endpoint))),
        ),
        seq(
          field('name', $.setting_name),
          optional(seq(':', optional($._line_breaks), field('value', $._setting_value))),
        ),
      ),

    // A setting name is one identifier: `no_transaction` (§settings-lists).
    setting_name: ($) => $._name,

    _setting_value: ($) =>
      choice(
        $.string,
        $.signed_number,
        $.number,
        $.boolean,
        $.null,
        $.expression,
        $.ident_group,
        $._name,
      ),

    // Parenthesized identifier list: only: (index, show) (volt §settings);
    // order lists allow a one-word modifier: order: (year desc, id) (§selects-over-groups).
    // It stands inside a settings list, so a line break between its items is
    // whitespace (§line-structure-and-whitespace).
    ident_group: ($) =>
      seq(
        '(',
        optional($._line_breaks),
        $._ident_item,
        repeat(seq(optional($._line_breaks), ',', optional($._line_breaks), $._ident_item)),
        optional($._line_breaks),
        ')',
      ),

    _ident_item: ($) => seq($._name, optional(alias($._name, $.ident_modifier))),

    signed_number: ($) => seq('-', $.number),

    // ==================== Terminals ====================

    _name: ($) => $.identifier,

    // A Go identifier: a Unicode letter or underscore, then letters, decimal
    // digits and underscores (§identifiers, §unicode-general-categories).
    identifier: () => /[\p{L}_][\p{L}\p{Nd}_]*/,

    // The one name that begins with digits: four or more, two underscores,
    // a label (§migrations).
    migration_name: () => /[0-9]+__[\p{L}\p{Nd}_]+/,

    // Single-line ("...") and multi-line ("""...""") strings, with the four
    // escapes \\, \", \n and \t (§string-literals, §multi-line-string-literals,
    // §escape-sequences). Contents are separate tokens so editors can inject
    // Markdown.
    string: ($) =>
      choice(
        seq(
          '"""',
          optional(field('content', alias($._triple_string_content, $.string_content))),
          '"""',
        ),
        seq(
          '"',
          optional(field('content', alias($._single_string_content, $.string_content))),
          token.immediate('"'),
        ),
      ),

    _single_string_content: () =>
      token.immediate(/([^"\\\n]|\\["\\nt])+/),

    _triple_string_content: () =>
      token.immediate(/([^"\\]|\\["\\nt]|"([^"\\]|\\["\\nt])|""([^"\\]|\\["\\nt]))+/),

    // Backtick expression: opaque SQL, no escapes, may span lines (§expression-literals).
    expression: ($) =>
      seq(
        '`',
        optional(field('content', alias(token.immediate(/[^`]+/), $.expression_content))),
        token.immediate('`'),
      ),

    number: () => /[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?/,

    boolean: () => choice(kw('true'), kw('false')),

    null: () => kw('null'),

    comment: () =>
      token(
        choice(
          seq('//', /[^\n]*/),
          seq('/*', /([^*]|\*+[^*/])*\*+/, '/'),
        ),
      ),

    _newline: () => /\n/,
  },
});
