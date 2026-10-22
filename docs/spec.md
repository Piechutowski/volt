# The Volt Language — Specification

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

**Status:** Normative for the implementation as it stands. The
implementation in this repository MUST be 100% compliant with this
document; where they disagree, one of the two has a bug, and the
commit that fixes it says which. Normative does not mean settled: the
language is alpha and any part of it may change, and it changes by
changing this document.

Volt is **one language**: a declarative complement to Go. This
specification has two parts and the appendices:

- **[Part I — The Schema Core](#part-i-the-schema-core)**: lexis,
  grammar and static semantics of the schema language: tables,
  columns, indexes, refs, types, partials, records and notes,
  with Go's types as the column types.
- **[Part II — The Project and Routing Layer](#part-ii-the-project-and-routing-layer)**:
  projects, packages, imports, pipelines, scopes, routes, resources,
  and the query layer.
- **[Mapping to Go](#mapping-to-go)** and **[Mapping to SQLite DDL](#mapping-to-sqlite-ddl)**:
  the generation contracts of the data layer (informative in
  form, pinned by generator goldens in force).

Every construct is specified by (1) a grammar production in EBNF, (2)
an enumerated list of constraints, and (3) a minimal example. Every
constraint is executable: the conformance corpus under
[`lang/conformance/snippets/`](../lang/conformance/snippets/) tags each snippet with the
section it exercises (`// spec: §…`); `valid/` MUST be accepted,
`invalid/` MUST be rejected, and `go test ./...` runs the whole chain.
The examples run too (§conformance-and-the-proof-chain): a `volt`
block is a whole file the front end accepts, a block whose fence reads
`volt invalid` one it rejects, and a block whose fence reads
`volt fragment` a piece of a file, shown without its package clause or
the Go functions and packages it names, that parses alone.
Diagnostics cite the section they enforce. The lint annex — legal but
suspicious schemas, every rule with executable examples — is
[`lint.md`](lint.md); lint rules never change what conforming Volt is.

---

# Part I — The Schema Core

**Status:** Normative.
Part I is Volt's schema language: tables, columns, indexes, refs,
types, partials, records and notes, with column types drawn
from Go (§columns, §types). A Part I file
body is a valid Volt file body (§layers-files-and-the-superset-rule).
The reference implementation is [`lang/`](../lang/); the executable
corpus is [`lang/conformance/snippets/`](../lang/conformance/snippets/),
where the snippets without a `package` clause are Part I's
(§conformance-and-the-proof-chain).

The schema language is declarative: it defines tables, columns,
indexes, constraints, relationships, sample data and
documentation metadata, and leaves storage to the generators
(§mapping-to-sqlite-ddl).

This document is a complete, formal specification of Volt's schema language intended for
implementers of parsers, compilers, and tooling. Every construct is specified
by (1) a grammar production in EBNF, (2) an enumerated list of constraints, and
(3) a minimal example. The collected grammar appears in [Collected grammar (Part I)](#collected-grammar-part-i).

---

## Table of Contents

- **Part I — The Schema Core**
  - [Notation](#notation)
    - [EBNF](#ebnf)
    - [Character Notation (U+XXXX)](#character-notation-uxxxx)
    - [Unicode General Categories](#unicode-general-categories)
    - [Case Sensitivity and Terminology](#case-sensitivity-and-terminology)
  - [Source Text](#source-text)
  - [Lexical Structure](#lexical-structure)
    - [Tokenization](#tokenization)
    - [Line Structure and Whitespace](#line-structure-and-whitespace)
    - [Comments](#comments)
    - [Identifiers](#identifiers)
    - [Keywords](#keywords)
    - [String Literals](#string-literals)
    - [Multi-line String Literals](#multi-line-string-literals)
    - [Escape Sequences](#escape-sequences)
    - [Numeric Literals](#numeric-literals)
    - [Boolean and Null Literals](#boolean-and-null-literals)
    - [Expression Literals](#expression-literals)
    - [Operators and Punctuation](#operators-and-punctuation)
  - [Common Syntactic Forms](#common-syntactic-forms)
    - [Names and Qualified Names](#names-and-qualified-names)
    - [Settings Lists](#settings-lists)
  - [Program Structure](#program-structure)
  - [Element Definitions](#element-definitions)
    - [Project](#project)
    - [Table](#table)
    - [Columns](#columns)
    - [Default Values](#default-values)
    - [Indexes](#indexes)
    - [Relationships (Ref)](#relationships-ref)
    - [Types](#types)
    - [TablePartial](#tablepartial)
    - [Records (Sample Data)](#records-sample-data)
    - [Notes](#notes)
  - [Static Semantics](#static-semantics)
    - [Namespaces and Uniqueness](#namespaces-and-uniqueness)
    - [Reference Resolution](#reference-resolution)
    - [Partial Injection Order](#partial-injection-order)
    - [Nullability and Cardinality](#nullability-and-cardinality)
  - [Collected grammar (Part I)](#collected-grammar-part-i)
- **Part II — The Project and Routing Layer**
  - [Layers, files and the superset rule](#layers-files-and-the-superset-rule)
  - [Projects and packages](#projects-and-packages)
  - [Imports](#imports)
  - [Pipelines](#pipelines)
  - [Scopes and routes](#scopes-and-routes)
    - [Route paths](#route-paths)
    - [Routes](#routes)
    - [Handler references](#handler-references)
    - [Query routes](#query-routes)
    - [Formats](#formats)
    - [Generated client](#generated-client)
    - [Event routes](#event-routes)
    - [Scopes](#scopes)
    - [Route names and reverse URLs](#route-names-and-reverse-urls)
    - [Route conflicts](#route-conflicts)
  - [Resources](#resources)
    - [Declaration](#declaration)
    - [The action table](#the-action-table)
    - [Settings](#settings)
    - [Table resolution](#table-resolution)
    - [Default handlers](#default-handlers)
  - [Datasets](#datasets)
    - [Dataset declaration](#dataset-declaration)
    - [Dataset settings](#dataset-settings)
  - [Settings whitelists](#settings-whitelists)
  - [Generation contract (informative)](#generation-contract-informative)
  - [Groups](#groups)
  - [Predicates](#predicates)
  - [Selects over groups](#selects-over-groups)
  - [Validators](#validators)
  - [Changesets](#changesets)
  - [Migrations](#migrations)
    - [The fold](#the-fold)
    - [Migration ledger](#migration-ledger)
    - [Migration tooling](#migration-tooling)
  - [Collected grammar (Part II)](#collected-grammar-part-ii)
  - [Conformance and the proof chain](#conformance-and-the-proof-chain)
- **Appendices**
  - [Mapping to Go](#mapping-to-go)
    - [Go types](#go-types)
    - [Nullability](#nullability)
    - [Names](#names)
    - [Notes become doc comments](#notes-become-doc-comments)
    - [Struct tags](#struct-tags)
    - [Generated query surface](#generated-query-surface)
  - [Mapping to SQLite DDL](#mapping-to-sqlite-ddl)
  - [Non-goals](#non-goals)
  - [License and Provenance](#license-and-provenance)

Sections are named, not numbered. A citation is `§` followed by the
heading's anchor, the heading lowercased with its words joined by
hyphens: `§columns`, `§selects-over-groups`, `§struct-tags`. A
citation never carries a constraint's number; numbers move, names
stay. Diagnostic codes are
`spec/<anchor>`. A citation that names no heading fails the build
(`lang/spec_doc_test.go`).

## Notation

### EBNF

The grammar is written in **EBNF** (Extended Backus–Naur Form) following
ISO/IEC 14977. Nonterminal names are lowercase words separated by spaces
(e.g. `table name`); the space is part of the name, not concatenation.

| Notation      | Meaning                                                        |
|---------------|----------------------------------------------------------------|
| `name = … ;`  | Rule: the nonterminal `name` is defined as `…`                 |
| `"text"`, `'text'` | Terminal: the literal character sequence `text`           |
| `,`           | Concatenation: `a, b` means `a` followed by `b`                |
| `\|`          | Alternation: `a \| b` means `a` or `b`                         |
| `[ x ]`       | Option: zero or one occurrence of `x`                          |
| `{ x }`       | Repetition: zero or more occurrences of `x`                    |
| `x, { x }`    | Idiom for one or more occurrences of `x`                       |
| `n * x`       | Exactly `n` occurrences of `x` (e.g. `4 * hex digit`)          |
| `( … )`       | Grouping                                                       |
| `x - y`       | Exception: anything matching `x` that does not match `y`       |
| `? … ?`       | Special sequence: prose description of a match                 |
| `(* … *)`     | Comment inside the grammar                                     |

Note that the square brackets, braces, and parentheses of the *EBNF
meta-language* are distinct from the language's tokens `[ ]`, `{ }`, `( )`, which
always appear quoted (`"["`, `"{"`, `"("`) when they are part of the language
being defined.

Whitespace between symbols in a production is insignificant unless a
production explicitly references the `newline` nonterminal: one that
ends with `newline` lies on one line (§line-structure-and-whitespace). A line break is
whitespace everywhere else.

### Character Notation (U+XXXX)

`U+XXXX` is the standard Unicode notation for a single character (code
point), where `XXXX` is its number in hexadecimal. The characters referenced
by this specification:

| Notation | Character                                            |
|----------|------------------------------------------------------|
| `U+000A` | LINE FEED — the newline character, `\n`              |
| `U+000D` | CARRIAGE RETURN — `\r` (first half of Windows CRLF)  |
| `U+0009` | CHARACTER TABULATION — the tab character, `\t`       |
| `U+0020` | SPACE — the ordinary space character                 |
| `U+0000` | NUL                                                  |
| `U+0008` | BACKSPACE                                            |
| `U+000B` | LINE TABULATION (vertical tab)                       |
| `U+000C` | FORM FEED                                            |

### Unicode General Categories

Unicode assigns every character a *General Category*. This specification
uses two of them to define identifiers:

- **Category L (Letter)** — letters of any script: `a`–`z`, `A`–`Z`, but
  also `ż`, `é`, `ß`, `я`, `漢`, `ا`, etc. (It is the union of the
  subcategories Lu uppercase, Ll lowercase, Lt titlecase, Lm modifier,
  Lo other.)
- **Category M (Mark)** — combining marks: characters that attach to the
  preceding character, such as a combining acute accent (U+0301). These are
  included so that accented text in *decomposed* form — where `é` is stored
  as `e` followed by U+0301 — is still a valid identifier.

In regular-expression terms these are `\p{L}` and `\p{M}`. Any Unicode-aware
implementation language provides them; an implementation MUST NOT
approximate category L with ASCII `[a-zA-Z]`.

### Case Sensitivity and Terminology

**Case sensitivity.** All *keywords* (terminals spelled with letters in
this grammar, e.g. `"Table"`, `"pk"`, `"null"`) are matched
**case-insensitively**: `Table`, `table`, and `TABLE` are equivalent.
User-defined names (identifiers) preserve case; implementations MUST treat
them case-sensitively for lookup unless stated otherwise.

**Terminology.** The key words MUST, MUST NOT, SHOULD, and MAY are to be
interpreted as described in RFC 2119.

---

## Source Text

1. A Volt source file is a sequence of Unicode characters, conventionally
   encoded as UTF-8, using the file extension `.volt` (§layers-files-and-the-superset-rule).
2. A *program* (§program-structure) is the content of one source file. Multiple files are
   related only through packages (§projects-and-packages).

---

## Lexical Structure

### Tokenization

Before parsing, the input is split into *tokens* (identifiers, literals,
operators, punctuation) by a scanner governed by two rules:

1. **Left to right, single pass.** The scanner starts at the first character
   and repeatedly cuts the next token off the front of the remaining input.
   It never backs up to re-tokenize text it has already consumed.
2. **Longest match** (also called *greedy* or *maximal munch*). If, at the
   current position, the upcoming characters could form more than one valid
   token, the scanner always chooses the **longest** one.

Consequences of the longest-match rule:

- `'''` opens a multi-line string, never the empty string `''` followed by `'`.
- `//` begins a comment, never two `/` operators; `<=`, `>=` and `!=`
  are one token each, never two.
- `user_id2` is one identifier, not the identifier `user_id` followed by the
  number `2` — the scanner keeps consuming while characters can extend the
  current token.

To write two adjacent tokens that would otherwise fuse, separate them with
whitespace.

### Line Structure and Whitespace

```ebnf
newline = ? U+000A LINE FEED ? ;
sp      = ? U+0020 SPACE ? | ? U+0009 TAB ? ;
```

1. Carriage return (U+000D) is discarded wherever it appears and produces no
   token; files with Windows (CRLF) line endings are therefore handled
   transparently. It keeps its place in the source: a tool that reports
   positions counts its byte in offsets and gives it no column, so every
   position points into the file as written.
2. Volt is **newline-sensitive**: a line break terminates statements such as
   column definitions, record rows, and settings-free field
   lines. Productions in this specification reference `newline` explicitly
   wherever it is syntactically significant. A production that ends with
   `newline` is **line-oriented**: it lies on one line, and a line break
   inside it is whitespace only inside brackets it opened (`[ ]`, `( )`,
   `{ }`), so a settings list may span lines and a column's type may not
   sit on the next line. The `newline` that ends it is also satisfied by
   the end of the file and by a following `}`, so a block's last line may
   close the block.
3. Space and tab characters separate tokens and are otherwise insignificant.
4. Indentation is never significant (except inside multi-line strings, §multi-line-string-literals),
   with the one exception of element boundaries.
5. **Element boundaries.** An unquoted identifier (§identifiers) in the first column
   of a line begins a top-level element (§program-structure) whenever it lies outside every
   string, comment and brace: the brace depth counts `{` up and `}` down,
   never below zero, over the tokens before it. The element before such an
   identifier ends there, complete or not: a parser MUST NOT read past it to
   finish an element, and MAY therefore parse each element from its own
   text. An element that continues onto a further line either indents that
   line or continues inside braces or a multi-line string; a settings list,
   an import block or a relationship that spills onto a first-column
   identifier line is a syntax error at that line.
6. **Recovery.** A syntax error inside an element discards the rest of
   that element, up to the next element start, and the next
   element is parsed as if the broken one were absent. Inside a braced
   or parenthesized body of items (a table's columns, a scope's
   routes, an import block's entries, and every other
   one-per-line body), a syntax error discards the rest of the item's
   line, up to the next line or the body's closing bracket, and the
   following items are parsed. The broken item is absent from the body,
   and a body that lost an item this way is never reported empty
   (§table, §migrations). What a conforming front end reports
   for invalid input is therefore determined by the input alone; the
   conformance corpus pins every diagnostic of every invalid snippet.

```volt invalid
Table users {
  id int [pk]
}

Table posts {
  id int [pk]
  author_id int
}

Ref: posts.author_id >
users.id                      // error: a new element begins here
```

### Comments

There are **exactly two** comment forms:

```ebnf
comment            = line comment | block comment ;
line comment       = "//", { any char - newline } ;
block comment      = "/*", block comment body, "*/" ;
block comment body = { any char } - ( { any char }, "*/", { any char } ) ;

any char           = ? any Unicode character ? ;
```

1. A line comment extends to, but does not include, the next `newline` (or
   end of file).
2. A block comment may span any number of lines and MUST be terminated by
   `*/` before end of file; an unterminated block comment is a lexical error.
3. Block comments do not nest.
4. Comments are trivia: they may appear between any two tokens and have no
   semantic effect. A line comment does **not** consume the terminating
   newline; the newline retains its statement-terminating role.

```volt
// single-line comment
/* block
   comment */
```

### Identifiers

```ebnf
identifier        = plain identifier | quoted identifier ;

letter            = ? any character of Unicode category L (Letter) ?
                  | ? any character of Unicode category M (Mark) ?
                  | "_" ;
digit             = "0" | "1" | "2" | "3" | "4"
                  | "5" | "6" | "7" | "8" | "9" ;
ident char        = letter | digit ;

plain identifier  = letter, { ident char }
                  | ( digit, { ident char } ) - number ;    (* digits alone are a number *)

quoted identifier = '"', { qi char | escape sequence }, '"' ;
qi char           = any char - ( '"' | "\" | newline ) ;
```

1. A plain identifier is a maximal run of letters, digits, combining marks,
   and underscores.
2. A plain identifier MAY begin with digits, but a token consisting only of
   digits (and at most one `.`) is a numeric literal (§numeric-literals), never an
   identifier. E.g. `2fa_codes` is an identifier; `255` is a number.
3. A quoted identifier (`"double quoted"`) permits any characters except an
   unescaped `"` and a line break, and supports the escape sequences of §escape-sequences.
   Use it for names containing spaces or other special characters.
4. Plain and quoted identifiers are interchangeable everywhere an
   `identifier` is expected; `users` and `"users"` denote the same name.

### Keywords

Volt has **no reserved words**. All keywords are contextual: a keyword such
as `Table` acts as a keyword only in keyword position and remains usable as an
ordinary identifier elsewhere (e.g. a column may be named `table`).

Element-type keywords: `Project`, `Table`, `TablePartial`, `Type`,
`Ref`, `Records`, `Migration`, `indexes`, `validators`.
Clause keyword: `from`.
Value keywords: `true`, `false`, `null`.
All are case-insensitive (§case-sensitivity-and-terminology).

The words of a migration body (`up`, `down`, `create`, `drop`, `rename`,
`alter`, `rebuild`, `exec`, `table`, `index`, `add`, `to`) are keywords
inside a `Migration` only (§migrations); elsewhere they are identifiers,
so a column may be named `up` or `to`.

### String Literals

```ebnf
string             = single line string | multi line string ;

single line string = "'", { sls char | escape sequence }, "'" ;
sls char           = any char - ( "'" | "\" | newline ) ;
```

1. A single-line string is delimited by single quotes `'` and MUST NOT
   contain an unescaped line break.
2. Escape sequences (§escape-sequences) are interpreted.

### Multi-line String Literals

```ebnf
multi line string = "'''", mls body, "'''" ;
mls body          = { ( any char - "\" ) | escape sequence }
                    - ( { any char }, "'''", { any char } ) ;
```

1. Delimited by triple single quotes `'''`; may span any number of lines.
2. Escape sequences (§escape-sequences) are interpreted; escape a literal `'` as `\'` and
   a literal `\` as `\\`.
3. A backslash immediately before a line break is a **line continuation**:
   both the backslash and the line break are removed from the value.
4. **Indentation stripping:** after escape processing, compute the minimum
   number of leading spaces over all non-empty lines; remove exactly that
   many leading spaces from every line. A first line that is empty (the
   common case, where content starts on the line after the opening `'''`)
   and a trailing newline before the closing `'''` are removed.

```volt
Table posts [note: '''
  This is a block string.
  It spans multiple lines.
'''] {
  id int [pk]
}
```

The note's value is `This is a block string.\nIt spans multiple lines.`

### Escape Sequences

Escape sequences apply inside single-line strings, multi-line strings, and
quoted identifiers. They do **not** apply inside expression literals (§expression-literals)
or comments.

```ebnf
escape sequence = "\", escaped item ;
escaped item    = "t" | "n" | "r" | "0" | "b" | "v" | "f"
                | "\" | "'" | '"' | "`"
                | newline
                | "u", 4 * hex digit
                | any char - "u" ;
hex digit       = digit
                | "a" | "b" | "c" | "d" | "e" | "f"
                | "A" | "B" | "C" | "D" | "E" | "F" ;
```

| Sequence | Value                                   |
|----------|-----------------------------------------|
| `\t`     | horizontal tab (U+0009)                 |
| `\n`     | line feed (U+000A)                      |
| `\r`     | carriage return (U+000D)                |
| `\0`     | NUL (U+0000)                            |
| `\b`     | backspace (U+0008)                      |
| `\v`     | vertical tab (U+000B)                   |
| `\f`     | form feed (U+000C)                      |
| `\\`     | backslash                               |
| `\'`     | single quote                            |
| `\"`     | double quote                            |
| `` \` `` | backtick                                |
| `\` + newline | nothing (line continuation)        |
| `\uHHHH` | the code unit U+HHHH (exactly 4 hex digits; fewer is an error) |
| `\c` (any other `c`) | the character `c` itself     |

### Numeric Literals

```ebnf
number   = digit, { digit }, [ ".", digit, { digit } ], [ exponent ] ;
exponent = ( "e" | "E" ), [ "+" | "-" ], digit, { digit } ;
```

1. Examples: `42`, `3.14`, `1e2`, `1.5e10`, `3.14e-5`.
2. A leading sign is not part of the literal; a negative value such as
   `-100` in record rows is parsed as prefix operator `-` applied to a number.
3. At most one decimal point is permitted. A digit run followed by further
   letters (e.g. `2fa`) lexes as an identifier (§identifiers); a digit run containing
   a dot followed by letters (e.g. `12.3abc`) is a lexical error.

### Boolean and Null Literals

```ebnf
boolean = "true" | "false" ;
null    = "null" ;
```

Case-insensitive, as all keywords.

### Expression Literals

```ebnf
expression literal = "`", { any char - "`" }, "`" ;
```

1. Delimited by backticks. The content is an opaque, raw SQL expression:
   backslash is **not** an escape character and there is no way to embed a
   literal backtick.
2. May span multiple lines.
3. Used for computed defaults (`` default: `now()` ``), expression indexes,
   and expression values in records.

### Operators and Punctuation

```ebnf
punct = "{" | "}" | "[" | "]" | "(" | ")"
      | "," | ":" | "." | "~" | "*" | "/"
      | "<" | ">" | "-" | "+" | "\" | "="
      | "!=" | "<=" | ">=" ;
```

These are every token that is neither a name nor a literal; any other
character outside a string, an identifier or a comment, `;` and `#`
among them, is a lexical error. Their roles:

| Token   | Role                                                        |
|---------|-------------------------------------------------------------|
| `{ }`   | element bodies                                              |
| `[ ]`   | settings lists, and the `[]` of the type `[]byte`           |
| `( )`   | composite column lists, records column lists, group sets (§groups), select projections (§selects-over-groups), import blocks (§imports) |
| `:`     | a setting value, a project property, the body of `Ref:`, a route parameter (§route-paths), a query parameter (§predicates) |
| `,`     | separator inside `[ ]`, `( )` and record rows               |
| `.`     | qualification: `table.column`, `pkg.Name` (§types, §scopes), `pkg.name` (§imports) |
| `~`     | TablePartial injection prefix                               |
| `>`     | a relationship, foreign key first (§relationships-ref); a comparison in a predicate (§predicates) |
| `-`     | the sign of a negative number                               |
| `*`     | every column of a select's target (§selects-over-groups)    |
| `/`     | route paths (§route-paths) and import paths (§imports)      |
| `+ \`   | group union and difference (§groups)                        |
| `= != < <= >=` | comparisons in a predicate (§predicates)              |

---

## Common Syntactic Forms

### Names and Qualified Names

```ebnf
name = identifier ;
```

1. A table, a partial and a column are each named by one `name`, and a
   type of the package by one identifier (§types); no name is
   qualified by a schema or any other container.
2. The qualifications Volt has are `table.column` in a relationship
   (§relationships-ref), `pkg.Name` naming a Go type (§types),
   `pkg.name` naming a declaration of an imported package (§imports),
   and `pkg.Name` naming a function of the package itself in a setting
   that takes one (§scopes). A dotted name is neither a default value
   (§default-values) nor a record value (§records-sample-data).

### Settings Lists

Settings attach metadata to the construct they follow. A settings list is
always delimited by square brackets and comma-separated:

```ebnf
settings      = "[", setting, { ",", setting }, "]" ;
setting       = setting name, [ ":", setting value ] ;
setting name  = identifier ;
setting value = string | number | boolean | null
              | expression literal | identifier ;
```

1. A setting is either a flag (`pk`, `unique`, `null`) or a key-value
   pair (`note: 'text'`, `default: 123`).
2. Setting names are case-insensitive. A setting name is one
   identifier, and so is a setting value that is a word; a name or
   value of several words joins them with underscores
   (`no_transaction`, §migrations; `set_null`, §relationships-ref).
   An identifier where a setting ends, before the next comma or the
   closing bracket, is a syntax error, since settings are separated by
   commas.
3. Each element section below enumerates *which* settings are valid there;
   a setting not listed for a construct is invalid in that position.
   The value of `ref:` is an endpoint, `table.column`, wherever `ref:`
   is valid (§columns, §migrations).
4. Within one settings list, each setting MUST appear at most once. The
   only exception is the column setting `tag` (§columns), which MAY be
   repeated.
5. A settings list MUST appear on the same line as (the end of) the
   construct it modifies.

---

## Program Structure

```ebnf
program = { element } ;

element = project
        | table
        | table partial
        | type decl
        | ref element
        | records element ;
```

1. A program is a sequence of top-level elements, in any order,
   separated by line breaks. Where one element ends
   and the next begins is lexical (§line-structure-and-whitespace): an unquoted identifier in the
   first column outside every brace, string and comment.
2. Forward references are permitted: an element may reference another
   element defined later in the file, or in another file of its package
   (§projects-and-packages). The language is fully declarative;
   declaration order carries no semantics.

---

## Element Definitions

### Project

Declares project-level metadata. There MUST be at most one `Project` element
per compiled schema.

```ebnf
project          = "Project", [ name ], [ project settings ],
                   "{", project body, "}" ;
project settings = "[", "note", ":", string, "]" ;
project body     = { project property } ;
project property = identifier, ":", string, newline ;
```

1. `name` names the project; it MAY be omitted.
2. Each property is a free-form key with a string value, one per line.
   The conventional well-known key is `database_type` (e.g. `'PostgreSQL'`,
   `'MySQL'`). Implementations MUST accept arbitrary property keys.
3. The `note:` setting (§notes) documents the project.

```volt
Project ecommerce [note: 'E-commerce database schema'] {
  database_type: 'PostgreSQL'
}
```

### Table

```ebnf
table          = "Table", name, [ table settings ],
                 "{", table body, "}" ;

table body     = { column
                 | indexes block
                 | validators block
                 | partial injection
                 | records block
                 } ;

table settings = "[", table setting, { ",", table setting }, "]" ;
table setting  = "note", ":", string
               | "model", ":", string ;
```

1. A table MUST contain at least one column, either directly or via an
   injected TablePartial (§tablepartial).
2. **Name.** A table has one name, a single `name`: there is no alias
   and no schema qualifier. Tables and groups share one namespace in a
   package (§groups).
3. **Settings.** `note` (§notes) and `model` are the only table
   settings. `model:` pins the Go model name used by `volt gen` (by
   default the singularized table name, §names); the `vet` rule
   [`modelname`](lint.md#modelname) tells you when it is needed.
4. Column names MUST be unique within a table after partial injection (§partial-injection-order).

```volt
Table users [note: 'Registered accounts'] {
  id int [pk]
  email string [unique]
}
```

### Columns

```ebnf
column          = name, column type, [ column settings ], newline ;

column type     = [ "[", "]" ], type name ;
type name       = [ identifier, "." ], identifier ;

column settings = "[", column setting, { ",", column setting }, "]" ;
column setting  = "pk"
                | "null"
                | "unique"
                | "increment"
                | "required"
                | "default", ":", default value
                | "note", ":", string
                | "tag", ":", string
                | "ref", ":", name, ".", name ;   (* table.column, §relationships-ref *)
```

1. **Type.** A column type is a Go type, matched case-sensitively, from
   a closed set: the built-ins `bool`, `int`, `float64`, `string`,
   `[]byte` and `time.Time`, or a type declared with `Type` (§types). Anything else is an error naming the column and
   the set; the `[]` of the grammar belongs to `[]byte` alone, so
   `[]string` is an unknown type. An unknown type is reported once, at
   the column, even when a TablePartial injects the column into several
   tables (§tablepartial): a select, a validator or a route that uses
   the column reports nothing further about its type. A select that
   uses such a column, in its predicate, its order, its projection or
   as a column its row type keeps, is kept unresolved: it generates no
   method, and the query routes and datasets that name it are checked
   as for any select (§query-routes, §dataset-declaration). A query
   route to a default CRUD method keyed by such a column, and a
   resources declaration over a table whose primary key is one, are
   kept and checked the same way (§query-routes, §table-resolution).
   One check is narrowed: a path parameter that a dropped predicate
   term over the column names, and none of the select's remaining
   parameters does, is not reported, since it belongs to the dropped
   term. An error that does not depend on the type, such as a group
   member that lacks the column, an `order:` term without a direction,
   a route conflict, a setting a route, resources or dataset does not
   take, or a validator function that does not exist, is reported as
   usual. A column whose type names a `Type` declaration in error
   reports nothing: the declaration's error (§types) stands for it.
   The type decides the Go field
   (§mapping-to-go), the SQLite column (§mapping-to-sqlite-ddl), the
   literals a `default:` accepts (§default-values), the empty value
   `required` refuses, and the comparisons a predicate allows
   (§predicates).
2. **Nullability.** A column is NOT NULL unless it carries `null`,
   which admits NULL. This mirrors Go, where a value is never nil unless
   its type says so. `[]byte` is the type that says so: its field is
   `[]byte` with or without `null`, and nil binds as NULL
   (§mapping-to-go), so a `[]byte` column without `null` refuses nil,
   its Go zero value, as a `not_null` field error when a write sends
   it (§changesets), and a non-nil empty slice stores empty bytes. A
   value of a declared type (§types) whose `driver.Valuer` returns nil
   binds as NULL the same way. A key column, one carrying `pk` or a
   member of a `[pk]` index (§indexes), MUST NOT carry `null`.
3. **`pk`** marks a single-column primary key. A composite primary key
   MUST be expressed as an index with the `pk` setting (§indexes). A
   table has at most one primary key, one column carrying `pk` or one
   `[pk]` index, counted over its columns and indexes after partial
   injection (§tablepartial): a second `pk` column, a `pk` column
   beside a `[pk]` index, or a second `[pk]` index is an error.
4. **`increment`** marks the column auto-increment: the database
   assigns its value, and a changeset has no setter for it
   (§changesets).

   > **Extension (this implementation).** Because the target is SQLite
   >, `increment` is honored in exactly one shape — the table's
   > single-column primary key of type `int`, which becomes the
   > rowid alias (§mapping-to-sqlite-ddl) — and is an **error** anywhere else: on a
   > non-integer column, on a column that is not the primary key, or
   > when the primary key is composite. A keyword that is typed MUST be
   > applied; silently ignoring it while no changeset ever marks the
   > column would insert NULL into a NOT NULL column at run time.
5. **`ref: table.column`** declares a relationship whose foreign key
   is this column (§relationships-ref). It appears at most once on a
   column (§settings-lists) and carries no referential actions; a
   relationship that needs `delete:` or `update:` is a `Ref` element.
6. Each column definition is terminated by a line break; the settings
   list is the only thing that follows the type.

> **Extension (this implementation).** The column setting `required`
> declares that a value must be present
> in the zero-value sense: a zero value that is set, or decoded into a
> row and passed to `From` (§changesets), satisfies a NOT NULL column,
> except one that binds as NULL (nil for `[]byte`, rule 2), so
> `required` is the word for "and not that either". It is the one built-in rule
> of `Validate` (§changesets), which the schema switches on per column,
> the field error with check `required`: it refuses `""` for `string`,
> `0` for `int` and `float64`, a zero length for `[]byte`, and for a
> declared type the empty value of its storage type. It is an error on a
> `null` column, which has no empty value to refuse since NULL is
> absence, on a `default:` or `increment` column, and on `bool` and
> `time.Time`, where every value is a value.
>
> **The `tag` setting.** The column setting `tag: string`
> (repeatable) passes one Go struct tag through to the generated field
> verbatim: every field property lives in
> the schema, and the passthrough serves every encoding and
> third-party library without minting a setting name per format.
>
> 1. The value MUST be exactly one `key:"value"` pair in Go struct-tag
>    form: a key containing no spaces, colons, double quotes or
>    backticks, then a colon, then a double-quoted value containing no
>    double quotes or backticks.
> 2. Within one column, tag keys MUST be unique across all `tag`
>    settings.
> 3. The key `db` is reserved and an error — the `db` tag is the scan
>    contract (§struct-tags).
> 4. A `json` tag replaces the generated default; every other tag is
>    appended after the generated pair, in declaration order
>    (§struct-tags).
>
> Note that `encoding/gob` ignores struct tags entirely: renaming a
> field for gob is a Go-field-name concern, not a tag concern, and has
> no surface here.

```volt
Table users {
  id        int [pk]
  user_name string [tag: 'json:"userName"', tag: 'xml:"user-name,attr"']
}
```

### Default Values

```ebnf
default value = [ "-" ], number
              | string
              | boolean
              | expression literal ;
```

| Kind          | Example                                               |
|---------------|-------------------------------------------------------|
| number        | `default: 123`, `default: -100`                       |
| string        | `default: 'direct'`                                   |
| boolean       | `default: false`                                      |
| expression    | `` default: `now() - interval '5 days'` ``            |

A bare identifier other than `true` and `false`, and a dotted name
such as `state.pending`, is **not** a valid default value.

1. **Typed.** A default MUST fit the column's type (§columns): an
   integer literal for `int`; a number for `float64`; a string for `string`;
   `true` or `false` for `bool`; for a declared type (§types), a
   literal of its storage type, so the default of a value set is the
   value as written (`default: 'pending'`). An expression literal is accepted on any column and is
   SQLite's to judge. `[]byte` and `time.Time` columns take only an
   expression literal. A default that does not fit is an error naming
   the column and its type.
2. **NULL has one spelling.** `null` is not a default value: a column
   that carries `null` and has no `default:` is NULL when a write
   leaves it out, so `default: null` would only say that twice and is
   an error.

### Indexes

An `indexes` block may appear inside a `Table` or `TablePartial` body. A
body MAY contain more than one `indexes` block; their contents accumulate.

```ebnf
indexes block  = "indexes", "{", { index }, "}" ;
index          = index key, [ index settings ], newline ;

index key      = index atom
               | "(", index atom, { ",", index atom }, ")" ;
index atom     = name | expression literal ;

index settings = "[", index setting, { ",", index setting }, "]" ;
index setting  = "name", ":", string
               | "unique"
               | "pk"
               | "where", ":", expression literal ;
```

1. An index key is a single column, a single expression, or a parenthesized
   composite of columns and/or expressions.
2. Every `name` in an index key MUST refer to a column of the enclosing
   table (after partial injection).
3. `pk` declares the index a (composite) primary key; `unique` a unique
   index. Combining `pk` and `unique` on one index is redundant but
   permitted. The members of a `pk` index are NOT NULL and MUST NOT
   carry `null` (§columns). A `pk` index is the table's one primary
   key: a table has at most one, and none beside a `pk` column
   (§columns). Its key is columns only; an expression atom in it is an
   error.
4. **Name.** An index without `name:` whose key is columns only is
   named `idx_<table>_<col>[_<col>...]` from the SQLite table name
   and its key columns in order (§mapping-to-sqlite-ddl), so that a
   schema and a migration (§migrations) name one index one way;
   `name:` overrides it. An index with an expression atom MUST carry
   `name:`, since no name derives from an expression. A `pk` index is
   a table constraint and has no name: `name:` on it is an error.
5. **`where`** takes an expression literal and makes the index
   partial: only the rows for which the condition holds are indexed,
   so the index stays small, and a unique partial index constrains
   only those rows. The DDL carries it as `WHERE <expr>`, verbatim and
   SQLite's to judge. `where:` on a `pk` index is an error, since a
   primary key covers every row.
6. An index carries no method and no note: SQLite has one index
   method, and an index documents itself by its key.

```volt
Table bookings {
  id int
  country string
  booking_date time.Time
  cancelled_at time.Time [null]

  indexes {
    (id, country) [pk]
    country [name: 'idx_country']
    (country, `lower(country)`) [unique, name: 'idx_bookings_country_ci']
    booking_date [where: `cancelled_at IS NULL`]
  }
}
```

### Relationships (Ref)

A relationship declares a foreign key. It has one direction and two
spellings: the `Ref` element, and the column setting `ref:` (§columns).
Both write the foreign key first.

```ebnf
ref element  = "Ref", ":", ref endpoint, ">", ref endpoint,
               [ ref settings ] ;

ref endpoint = name, ".", column group ;
column group = name
             | "(", name, { ",", name }, ")" ;

ref settings = "[", ref setting, { ",", ref setting }, "]" ;
ref setting  = "delete", ":", ref action
             | "update", ":", ref action ;
ref action   = "cascade" | "restrict" | "set_null" | "set_default" ;
```

1. **Direction.** The left endpoint is the foreign key and the right
   endpoint the columns it references: `Ref: posts.user_id > users.id`
   reads "posts.user_id references users.id". `>` is the only
   relationship operator, and a `Ref` has no name and no braced form:
   one relationship, one spelling.
2. **Inline form.** The column setting `ref: table.column` declares the
   same relationship with the declaring column as the foreign key, so
   `user_id int [ref: users.id]` in `posts` is
   `Ref: posts.user_id > users.id`. It names one column and carries no
   settings (§columns).
3. **Endpoints.** Each endpoint is a table name followed by one column
   or a parenthesized column group. In a composite relationship both
   endpoints MUST list the same number of columns:
   `Ref: a.(x, y) > b.(x, y)`.
4. **Foreign-key side.** The foreign key is the left endpoint of a
   `Ref` and the declaring column of a `ref:`, without exception: the
   checker, the lint rules, the DDL (§mapping-to-sqlite-ddl), the
   generated Go and the shapes of a migration (§migrations) all read
   it so.
5. **One-to-one.** There is no one-to-one operator. A one-to-one
   relationship is a foreign key that is also unique: `unique` on the
   foreign-key column, its `pk`, or a unique index over exactly the
   foreign-key columns, beside the reference, as in
   `user_id int [unique, ref: users.id]`. The uniqueness is the
   column's, and the DDL and the generated query surface treat it as
   any unique column's (§mapping-to-sqlite-ddl, §generated-query-surface).
6. **Many-to-many.** There is no many-to-many operator: a junction
   table with two foreign keys models it.
7. **Referential actions.** `delete:` and `update:` correspond to SQL
   `ON DELETE` / `ON UPDATE`: `cascade` is `CASCADE`, `restrict`
   `RESTRICT`, `set_null` `SET NULL` and `set_default` `SET DEFAULT`.
   SQLite's default, `NO ACTION`, has no spelling: it is what leaving
   `delete:` or `update:` out says. `set_null` is an error unless every
   foreign-key column of the relationship carries `null`, and
   `set_default` unless every one carries `null` or a `default:`
   (§columns, §default-values), since the database would otherwise
   write a NULL the column refuses.
8. Zero-or-one / zero-or-many cardinality is not written explicitly; it is
   derived from the nullability of the foreign-key column (§columns).
9. Duplicate relationships between the same column sets are invalid,
   whichever side is the foreign key: `a.x > b.y` and `b.y > a.x`
   relate the same columns.
10. A `Ref` element declares exactly **one** relationship; to declare
    several, write several `Ref` elements.

```volt
Table users {
  id int [pk]
}

Table posts {
  id int [pk]
  user_id int [null, ref: users.id]
  editor_id int [null]
}

Table profiles {
  user_id int [unique, ref: users.id]
}

Table merchants {
  id int
  country_code string
  indexes {
    (id, country_code) [pk]
  }
}

Table merchant_periods {
  merchant_id int
  country_code string
}

Ref: posts.editor_id > users.id [delete: set_null, update: cascade]

Ref: merchant_periods.(merchant_id, country_code) > merchants.(id, country_code)
```

### Types

A **Type** declaration admits a Go type of the application as a column
type. The schema names the Go type, where it comes from and how SQLite
stores it; the Go type does the rest.

```ebnf
type decl    = "Type", type name, storage type, [ "from", string ], newline ;
storage type = column type ;
```

1. **Name.** `pkg.Name` names the Go type `Name` of the package
   imported from the `from` path under the alias `pkg`; the path is a
   non-empty import path, and generated code imports it as
   `import pkg "path"`. The alias MUST NOT be a package name generated
   code imports for itself under another path: `context`, `driver`,
   `errors`, `fmt`, `gob`, `http`, `json`, `sql`, `strings` and `time`
   are free only with their standard path (`sql` from `database/sql`,
   `driver` from `database/sql/driver`, `json` from `encoding/json`,
   `gob` from `encoding/gob`, `http` from `net/http`, the rest from
   the path of their name), and `rt` and `volt` are never free. A bare
   `Name` without `from` names a type declared in a Go file of the data
   package itself (§projects-and-packages): the project pass refuses a
   bare `Name` that no Go file of the package declares, with an error
   naming the line to write (`type OrderState string`), while the
   schema pass, which reads no Go file, does not. `from` on a bare name
   is an error, as is `pkg.Name` without `from`.
   Declared names are unique within the package and MUST NOT spell a
   built-in type. A declared type names an exported Go type: `Name`,
   bare or after `pkg.`, MUST begin with an upper-case letter, since
   models other packages import carry it and no other package can
   name an unexported type; the import names and the unexported
   helpers of generated code are thereby never taken. A bare name
   shares the package scope with the exported names generated code
   declares (§names), so it MUST NOT be one of them: a model, a
   changeset with its aliases and constructor, a column handle, an
   option wrapper, the `Queries` handle or its constructor `New`, in a
   package that declares migrations `Migrations` and `Migrate`
   (§migration-tooling), and in a package that routes the router's
   `Controllers`, `NewRouter`, `Table`, each `<Name>Controller`
   interface and each `Path<Helper>` function
   (§route-names-and-reverse-urls); the error on a model's name
   suggests a name that is free. The type of a column is therefore
   named apart from the column's handle: `orders.status` mints the
   handle `OrderStatus`, so its type is `OrderState`.
2. **Storage.** The storage type is one of the six built-ins
   (§columns), never another declared type. It fixes the
   SQLite column (§mapping-to-sqlite-ddl), the empty value `required`
   refuses (§columns) and the literals a `default:` accepts
   (§default-values).
3. **Contract.** Generated code scans into a pointer and binds values,
   and the Go type decides how. A bare type whose Go declaration spells
   its storage type as its underlying type, or as the type it aliases
   (`type OrderState string`, `type Priority = int`), for storage
   `bool`, `int`, `float64` or `string`, is **kinded** (a generic
   declaration never is): generated code scans it through a pointer to
   its storage type (`(*bool)(&v.Featured)`), `rt.Null` converts a
   nullable one, and database/sql binds it by its kind, so it needs
   neither method and generated code asserts nothing for it.
   Every other declared type, every package-qualified one among them,
   MUST implement `database/sql.Scanner` on its pointer and
   `database/sql/driver.Valuer` on its value; generated code asserts
   both
   (`var _ sql.Scanner = (*pkg.Name)(nil)` and
   `var _ driver.Valuer = *new(pkg.Name)`), so a type that does not
   fails to compile at the assertion, naming the type. Storage
   `time.Time` and `[]byte` is never kinded: a struct does not convert
   by kind, and a named byte slice scans by reflection without copying
   the driver's buffer. The declaration is read by its spelling, as a
   validator's signature is (§validators), and the Go compiler remains
   the authority on what it denotes. A column of a declared type that
   carries `null` is `rt.Null[pkg.Name]`.
4. **In predicates.** A column of a declared type compares with `=`,
   `!=`, `in` and `is null` only (§predicates).
5. **Value sets.** A column that holds one of a fixed set of values is
   a column of a kinded type. Its values are Go constants and its
   membership is a validator (§validators), both written in the Go
   file that declares the type, so the user names the constants; its
   defaults and records are literals of the storage type. `<Model>New`
   seeds a literal default of a kinded type, `required` compares its
   value with the storage type's empty value directly (§changesets),
   and a list parameter binds as one JSON array of its values
   (§predicates). The database stores the storage type and carries no
   `CHECK` (§validators).
6. **Output elsewhere.** `volt gen -o DIR` (§projects-and-packages)
   writing Go for a package that declares a bare type, to a `DIR`
   other than the package's own directory, is an error naming the
   fix: the type lives in the package's own Go files, which do not
   travel to `DIR`, so it is declared in a package of its own and
   named `Type pkg.Name storage from 'path'`. `-o` naming the
   package's own directory writes where `gen` without it does.

```volt
Type money.Money int    from 'example.com/app/money'
Type png.Png     []byte from 'image/png'
Type Slug        string

Table products {
  id    int         [pk, increment]
  slug  Slug        [unique]
  price money.Money
  icon  png.Png     [null]
}
```

A value set (rule 5): the schema names the type and the validators,
and the Go file beside it declares the type, its constants and the
validators.

```volt
Type OrderState string

Table orders {
  id     int        [pk, increment]
  status OrderState [default: 'pending']
  prev   OrderState [null]

  validators {
    OrderStateKnown(status)
    OrderStateKnownNull(prev)
  }

  Records (id, status) {
    1, 'paid'
  }
}
```

```go
// OrderState is the state of an order (§types).
type OrderState string

const (
	OrderStatePending OrderState = "pending"
	// OrderStatePaid is settled by the processor.
	OrderStatePaid    OrderState = "paid"
	OrderStateShipped OrderState = "shipped"
)

// OrderStateKnown refuses a state outside the constants (§validators).
func OrderStateKnown(s OrderState) error {
	switch s {
	case OrderStatePending, OrderStatePaid, OrderStateShipped:
		return nil
	}
	return fmt.Errorf("%q is not an order state", string(s))
}

// OrderStateKnownNull is OrderStateKnown for a nullable column, which
// NULL passes (§validators).
func OrderStateKnownNull(s rt.Null[OrderState]) error {
	if !s.Valid {
		return nil
	}
	return OrderStateKnown(s.V)
}
```

Generated code types the field `Status OrderState` and the nullable
one `Prev rt.Null[OrderState]`, asserts nothing for `OrderState`,
starts `OrderNew()` from `Order{Status: "pending"}`, and records a
failed validator under its name, `OrderStateKnown`; the DDL is
`status TEXT NOT NULL DEFAULT 'pending'`. A select over the column
takes the type: `Select by_state for orders where status in :states`
mints `OrderByState(ctx, states []OrderState)`.

### TablePartial

A `TablePartial` declares a reusable fragment of a table body. Tables inject
partials by name.

```ebnf
table partial     = "TablePartial", name, "{", partial body, "}" ;
partial body      = { column
                    | indexes block
                    | validators block
                    } ;

partial injection = "~", name, newline ;
```

1. Partial names live in their own namespace and MUST be unique.
2. A partial body is a table body without `records` and without nested
   partial injections. A partial takes no settings: a table's `note`
   and `model` are its own.
3. An injection `~p` inside a table body replaces itself with the entire
   content of partial `p` at that position. Injection order is source
   order.
4. **Conflict resolution** when the same column is defined more than
   once:
   1. A column written directly in the table overrides any partial's.
   2. Otherwise the **last-injected** partial (in source order) wins.
5. **Indexes are not overridden.** An index a partial injects whose key
   is the key of an index of the table, or of an index another
   injection brings (another partial, or the same partial injected
   again), is an error: an index is declared once, in the table or in
   one partial. Two indexes on one key inside one partial are not this
   error; they are that partial's own redundancy, as two in a table
   body are. A partial injected twice lists each of its validators
   once.

```volt
TablePartial base_template {
  id int [pk]
  created_at time.Time [default: `now()`]
}

Table users {
  ~base_template
  name string
}
```

### Records (Sample Data)

`Records` declares sample rows for a table, in CSV-like syntax. Records may
be declared at top level (naming the table) or inside a table body.

```ebnf
records element = "Records", name, records columns,
                  "{", { record row }, "}" ;

records block   = "Records", [ records columns ],
                  "{", { record row }, "}" ;

records columns = "(", name, { ",", name }, ")" ;

record row      = record value, { ",", record value }, newline ;
record value    = string
                | [ "-" ], number
                | boolean
                | null
                | expression literal ;
```

1. Top-level records MUST name the target table and MUST list columns.
   In-table records MAY omit the column list; the columns then default to
   all table columns in definition order (after partial injection, §partial-injection-order).
2. A table MUST have at most one records block.
3. Each row MUST supply exactly as many values as listed columns.
4. **Value typing.** Each value MUST fit the target column's type
   (§columns), as a default must (§default-values): an integer literal
   for `int`; a number for `float64`; a string for `string` and for
   `time.Time` (ISO 8601 text, `'2024-01-15 10:30:00'`); `true` or
   `false` for `bool`; for a declared type, a literal of its storage
   type (`'paid'` for a value set stored as `string`, §types). `null`
   is NULL and is allowed only on a column that carries `null`.
   A `[]byte` column takes an expression literal or NULL. An expression
   literal (backticks) is accepted on any column and is SQLite's to
   judge. A bare identifier other than `true`, `false` and `null`, and
   a dotted name, is **not** a value. A value that does not fit is an
   error naming the column and its type.
5. **No empty field.** Every field of a row holds a value: NULL is
   written `null`, and nothing between two commas, before the first or
   after the last is an error.
6. **Coverage.** A records column list MUST name every column of the
   table, injected columns included (§partial-injection-order), that
   is NOT NULL, has no `default:` and is not the rowid alias (`int
   [pk, increment]`, §mapping-to-sqlite-ddl), since the database
   would write NULL into such a column left out and refuse the row. An
   omitted list names every column (rule 1).

```volt
Type JobState string

Table users {
  id int [pk]
  name string [null]
  status JobState [null]

  Records (id, name, status) {
    1, 'Alice', 'created'
    2, 'Bob', 'running'
    3, null, null
  }
}

Table posts {
  id int [pk]
  user_id int [ref: users.id]
  posted_at time.Time
}

Records posts(id, user_id, posted_at) {
  1, 1, `CURRENT_TIMESTAMP`
  2, 2, '2024-01-15 10:30:00'
}
```

### Notes

A note is human-readable documentation. It has one spelling, the
`note:` setting, and one meaning: generated code carries it.

1. `note: string` is a setting of a `Project` (§project), a `Table`
   (§table) and a column (§columns), and of nothing else: a partial,
   a type, an index, a relationship, a group and a migration carry no
   note. A constant of a declared type documents itself in the Go
   file that declares it (§types).
2. The value is a string. A multi-line string
   (§multi-line-string-literals) is permitted and conventionally
   contains Markdown; a settings list spans lines inside its brackets
   (§line-structure-and-whitespace), so a long note stays readable.
3. `Note` is not a keyword: a `Note:` line in a table body reads as a
   column, and is an error there.
4. Every note becomes a doc comment in generated Go
   (§notes-become-doc-comments) and a comment in the DDL
   (§mapping-to-sqlite-ddl).

```volt
Table posts [note: '''
  A blog post.
  Its author sees it before anyone else.
'''] {
  id int [pk]
  title string [note: 'Shown in lists']
}
```

---

## Static Semantics

### Namespaces and Uniqueness

Within one package (§projects-and-packages):

1. Table names MUST be unique. Tables and groups share this namespace
   (§groups): a group MUST NOT take a table's name.
2. Declared Type names (§types) form their own namespace and MUST be
   unique. A bare one also shares the Go package scope with the names
   generated code declares and MUST NOT take one of them (§types).
3. TablePartial names form their own namespace and MUST be unique.
4. Column names MUST be unique within their table.

### Reference Resolution

1. Relationship endpoints, index columns and records columns MUST
   resolve to existing elements.
2. Composite relationship endpoints MUST have equal arity, and referenced
   column lists must match in count and order.

### Partial Injection Order

1. Injections are expanded in source order; the effective column order of a
   table is the concatenation of injected and direct columns in source
   order (this order also drives implicit records column lists, §records-sample-data).
2. Conflicts resolve per §tablepartial.

### Nullability and Cardinality

1. A column that carries `null` is nullable; every other column is
   NOT NULL (§columns).
2. A nullable foreign-key column yields zero-or-one / zero-or-many
   cardinality on the FK side; a NOT NULL one yields exactly-one.

---

## Collected grammar (Part I)

The complete grammar in EBNF (ISO/IEC 14977), collected from the sections
above.

```ebnf
(* ===== Program structure ===== *)

program              = { element } ;

element              = project | table | table partial
                     | type decl | ref element | records element ;

(* ===== Project ===== *)

project              = "Project", [ name ], [ project settings ],
                       "{", project body, "}" ;
project settings     = "[", "note", ":", string, "]" ;
project body         = { project property } ;
project property     = identifier, ":", string, newline ;

(* ===== Table, Columns, Default Values, Indexes ===== *)

table                = "Table", name, [ table settings ],
                       "{", table body, "}" ;
table body           = { column | indexes block | validators block
                       | partial injection | records block } ;
table settings       = "[", table setting, { ",", table setting }, "]" ;
table setting        = "note", ":", string
                     | "model", ":", string ;

column               = name, column type, [ column settings ], newline ;
column type          = [ "[", "]" ], type name ;
type name            = [ identifier, "." ], identifier ;
column settings      = "[", column setting, { ",", column setting }, "]" ;
column setting       = "pk" | "null"
                     | "unique" | "increment" | "required"
                     | "default", ":", default value
                     | "note", ":", string
                     | "tag", ":", string
                     | "ref", ":", name, ".", name ;
default value        = [ "-" ], number | string | boolean
                     | expression literal ;

indexes block        = "indexes", "{", { index }, "}" ;
index                = index key, [ index settings ], newline ;
index key            = index atom
                     | "(", index atom, { ",", index atom }, ")" ;
index atom           = name | expression literal ;
index settings       = "[", index setting, { ",", index setting }, "]" ;
index setting        = "name", ":", string
                     | "unique" | "pk"
                     | "where", ":", expression literal ;

(* ===== Relationships ===== *)

ref element          = "Ref", ":", ref endpoint, ">", ref endpoint,
                       [ ref settings ] ;
ref endpoint         = name, ".", column group ;
column group         = name | "(", name, { ",", name }, ")" ;
ref settings         = "[", ref setting, { ",", ref setting }, "]" ;
ref setting          = "delete", ":", ref action
                     | "update", ":", ref action ;
ref action           = "cascade" | "restrict" | "set_null" | "set_default" ;

(* ===== Types ===== *)

type decl            = "Type", type name, storage type, [ "from", string ], newline ;
storage type         = column type ;

(* ===== TablePartial ===== *)

table partial        = "TablePartial", name, "{", partial body, "}" ;
partial body         = { column | indexes block | validators block } ;
partial injection    = "~", name, newline ;

(* ===== Records ===== *)

records element      = "Records", name, records columns,
                       "{", { record row }, "}" ;
records block        = "Records", [ records columns ],
                       "{", { record row }, "}" ;
records columns      = "(", name, { ",", name }, ")" ;
record row           = record value, { ",", record value }, newline ;
record value         = string | [ "-" ], number | boolean | null
                     | expression literal ;

(* ===== Common forms ===== *)

name                 = identifier ;

settings             = "[", setting, { ",", setting }, "]" ;
setting              = setting name, [ ":", setting value ] ;
setting name         = identifier ;
setting value        = string | number | boolean | null
                     | expression literal | identifier ;

(* ===== Lexical grammar ===== *)

newline              = ? U+000A LINE FEED ? ;
sp                   = ? U+0020 SPACE ? | ? U+0009 TAB ? ;
any char             = ? any Unicode character ? ;

comment              = line comment | block comment ;
line comment         = "//", { any char - newline } ;
block comment        = "/*", block comment body, "*/" ;
block comment body   = { any char } - ( { any char }, "*/", { any char } ) ;

identifier           = plain identifier | quoted identifier ;
letter               = ? any character of Unicode category L (Letter) ?
                     | ? any character of Unicode category M (Mark) ?
                     | "_" ;
digit                = "0" | "1" | "2" | "3" | "4"
                     | "5" | "6" | "7" | "8" | "9" ;
ident char           = letter | digit ;
plain identifier     = letter, { ident char }
                     | ( digit, { ident char } ) - number ;
quoted identifier    = '"', { qi char | escape sequence }, '"' ;
qi char              = any char - ( '"' | "\" | newline ) ;

string               = single line string | multi line string ;
single line string   = "'", { sls char | escape sequence }, "'" ;
sls char             = any char - ( "'" | "\" | newline ) ;
multi line string    = "'''", mls body, "'''" ;
mls body             = { ( any char - "\" ) | escape sequence }
                     - ( { any char }, "'''", { any char } ) ;

escape sequence      = "\", escaped item ;
escaped item         = "t" | "n" | "r" | "0" | "b" | "v" | "f"
                     | "\" | "'" | '"' | "`"
                     | newline
                     | "u", 4 * hex digit
                     | any char - "u" ;
hex digit            = digit
                     | "a" | "b" | "c" | "d" | "e" | "f"
                     | "A" | "B" | "C" | "D" | "E" | "F" ;

number               = digit, { digit },
                       [ ".", digit, { digit } ], [ exponent ] ;
exponent             = ( "e" | "E" ), [ "+" | "-" ], digit, { digit } ;

boolean              = "true" | "false" ;
null                 = "null" ;

expression literal   = "`", { any char - "`" }, "`" ;
punct                = "{" | "}" | "[" | "]" | "(" | ")"
                     | "," | ":" | "." | "~" | "*" | "/"
                     | "<" | ">" | "-" | "+" | "\" | "="
                     | "!=" | "<=" | ">=" ;
```

---

---

# Part II — The Project and Routing Layer

**Status:** Normative.
Part I specifies the schema core; this part specifies everything Volt
adds above it. Both parts cite sections by heading name, never by
number, and a diagnostic cites the section it enforces as
`spec/<anchor>` (`spec/scopes-and-routes`).

Every construct below is specified by (1) a grammar production in EBNF
(notation of Part I §notation), (2) an enumerated list of constraints, and
(3) a minimal example. The collected grammar appears in
[Collected grammar (Part II)](#collected-grammar-part-ii). The executable companion
is the conformance corpus in [`lang/conformance/snippets/`](../lang/conformance/snippets/):
files under `valid/` MUST be accepted by a conforming implementation,
files under `invalid/` MUST be rejected.

---

## Layers, files and the superset rule

1. A Volt source file conventionally uses the extension **`.volt`**.
   Whether a file happens to use only the schema core (Part I) or the
   full language follows from its content, never from its name.
2. **Layering rule.** The full language is a superset of Part I: every
   valid Part I program is a valid Volt file body, and a file body
   means the same at every layer. Part II adds the package clause and
   its own declarations and takes nothing away. The lexical grammar is
   Part I §lexical-structure with one addition:

   ```ebnf
   slash = "/" ;
   ```

   `/` is a token wherever it is not the start of a `//` or `/*`
   comment. Consequence: **`/*` always opens a block comment**, which
   is why the rest-of-path wildcard is spelled `:name...` (§route-paths)
   and not `*name` — after a slash, a star-marked wildcard would open a
   block comment.
3. Declaration order and file assignment carry no semantics (§projects-and-packages).

## Projects and packages

```ebnf
package clause = "package", name, newline ;
```

1. A **project** is a Go module: the directory tree rooted at the
   nearest ancestor directory containing **`go.mod`**. Its
   `module` directive is the project's module path; every other
   directive is Go's business and ignored. There is no second manifest.
2. Every `.volt` file MUST begin with a package clause: its first
   declaration, at most one per file. The name is a plain (unquoted)
   identifier. The name `volt` is reserved: generated files import the
   volt runtime under that qualifier (§generation-contract-informative), so a package of that name
   is an error.
3. A **package** is a directory: all `.volt` files in one directory
   MUST declare the same package name, and share one namespace per
   element kind (tables, types, partials, pipelines, scopes, each per
   Part I §namespaces-and-uniqueness extended with §pipelines and §scopes-and-routes).
4. In every directory other than the project root, the package name
   MUST equal the directory's base name. The root directory's package
   may take any name.
5. **Layout invariance.** Splitting or merging the `.volt` files of a
   package, or reordering declarations, MUST NOT change the meaning of
   the package. Implementations MUST process a package as the
   concatenation of its files in file-name order, and route expansion
   (§route-conflicts) follows declaration order within that concatenation.
6. Directories whose name begins with `.` or `_`, and directories named
   `testdata` or `node_modules`, are not part of the project — Go's own
   exclusions. Neither is any subdirectory containing its own `go.mod`
   (a nested module is a different project; its packages belong to it).
7. **Loading is import-driven.** A tool loads the packages it is asked
   about and, transitively, the packages they import (§imports) — nothing
   else, so stray `.volt` trees elsewhere in the module never
   interfere. Packages are named as in Go: a directory names its
   package, `dir/...` names every package beneath `dir`, the excluded
   directories skipped, and no argument means the working directory. `volt gen`
   writes output for the named packages only; imports are loaded for
   checking, never written. An argument may also name a `.volt` file,
   which stands for its directory's package, so a `//go:generate` line
   can point at the file beside it.
8. **Output placement is the caller's.** `volt gen -o DIR` writes one
   named package's output into `DIR` instead of the package directory
   — any directory, another module's included, since a package that
   routes its own tables (§query-routes) imports nothing across packages —
   under the package clause `DIR`'s Go files declare (or `-package`),
   and `-parts` selects among `models`, `queries`, `router`, `client`
   and `sql`; the client then lands beside the models (§generated-client). A
   `.volt` file's own package clause stays mandatory: it
   names the package to the language and to the editor; only the
   clause of what is written follows the target. A package that
   declares a bare `Type` writes no Go to a `DIR` other than its own
   directory, since the type lives in the package's own Go files
   (rule 9, §types); `-parts sql` alone is unaffected.
9. **Go files.** A package's Go files are the non-test `.go` files of
   its directory that the go tool would compile for it (go/build's
   file constraints), less those carrying the generated marker. The Go
   names its `.volt` files use resolve there and nowhere else, each
   held to its spelled signature or declaration: validators
   (§validators), plugs (§pipelines), error handlers (§scopes) and bare
   declared types (§types). A Go file with a syntax error hides the
   declarations after the error, and a diagnostic about a missing
   declaration says so.

```volt
// db/schema.volt
package db

Table users {
	id int [pk, increment]
}
```

## Imports

```ebnf
import decl  = "import", "(", import entry, { import entry }, ")" ;
import entry = [ alias ], package path, newline ;
alias        = name ;
package path = name, { slash, name } ;
```

1. An import declaration is a parenthesized, newline-separated block of
   import entries; an empty block is an error. Import declarations appear
   after the package clause, before other use of the imported names.
2. A package path names a package by its directory path from the
   project root, `/`-separated, with no `./`, no `..`, and no interior
   whitespace; its segments, and the alias, are plain (unquoted)
   identifiers (§route-paths). The path MUST name an existing package of
   the project.
3. The imported package is referenced through its **qualifier**: the
   alias when given, else the last path segment. All cross-package
   references are qualifier-prefixed (`db.User`); nothing is ever
   imported into the local scope. A `Migration` is not an importable
   element: it belongs to the package whose tables it migrates, and
   nothing of another package may name it (§migrations).
4. Within one package (across all its files): importing the same path
   twice is idempotent, but with two different qualifiers it is an
   error; two different paths MUST NOT share a qualifier, nor may one
   take the package's own name, which qualifies its own declarations
   (§pipelines, §handler-references); a package MUST NOT import itself; **every import
   MUST be used** — an import whose qualifier is never referenced is
   an error.
5. **Import cycles are errors.** The package import graph MUST be
   acyclic.

```volt fragment
package app

import (
	db
	d2 shared/dicts
)
```

## Pipelines

```ebnf
pipeline = "Pipeline", name, "{", { plug }, "}" ;
plug     = "use", go ref, newline ;
go ref   = name, [ ".", name ] ;
```

1. A Pipeline is a named, ordered middleware list. Pipeline names form
   one namespace per package and MUST be unique. A pipeline body
   contains only plug lines; it MAY be empty.
2. A plug references Go middleware of type
   `func(http.Handler) http.Handler` by name: `volt.<Name>` (the volt
   runtime), `<Name>` or `<package name>.<Name>` (a function of the
   containing package). Referencing an *imported* package's function is
   an error suggesting a local wrapper.
   A function of the containing package MUST exist as a non-generic
   top-level function in the package directory's non-test Go files that
   the go tool would compile for it (go/build's file constraints: no
   leading `_` or `.`, a matching GOOS/GOARCH suffix, a satisfied
   `//go:build` line), with exactly one parameter spelled
   `http.Handler` and exactly one result spelled `http.Handler`:
   a missing function or another signature is an error naming the
   signature to write. The Go compiler still compiles the result; the
   checker catches the typo and the wrong shape first.
3. Pipelines contribute middleware in declaration order, outermost
   first, composed statically in generated code (§scopes).

```volt fragment
Pipeline api {
	use volt.RequestID
	use BearerAuth
}
```

## Scopes and routes

### Route paths

```ebnf
route path = slash,
             [ segment, { slash, segment } ] ;
segment    = name
           | ":", name, [ "(", param type, ")" ]
           | ":", name, ".", ".", "." ;  (* three '.' tokens, contiguous *)
param type = "int" | "string" ;
```

1. All tokens of a route path are **contiguous**: interior whitespace
   ends the path. A bare `/` is the root path. A non-root path MUST NOT
   end with `/` and MUST NOT contain an empty segment (`//`): routes
   match exactly (§scopes), so a trailing slash would spell a silently
   different pattern.
2. A `:name` segment captures one path segment as a parameter. The
   parameter name MUST be a valid, non-keyword Go identifier, MUST NOT
   be one of the reserved names `w`, `r`, `opts`, `volt`, `cs` (they appear
   in generated signatures and would be shadowed), and MUST be unique
   within the route's full path (scope prefixes included).
3. A parameter's type defaults to `string`; the closed type set is
   `int` and `string`, the Go types the model generator emits for
   routable primary keys.
   **Types shape handler signatures, never matching**: a request
   segment that fails to parse as the declared type is that route's
   404, not a fallthrough to a sibling route.
4. `:name...` captures the rest of the path (a **wildcard**), always as
   `string`. It MUST be the last segment of the full path, MUST NOT
   carry a type, and MUST NOT appear in a Scope prefix. (Spelling
   rationale: §layers-files-and-the-superset-rule.)
5. Grammar note: a path is assembled from ordinary tokens; the
   contiguity rule of clause 1 is what makes it one lexical island.
6. Every segment name — literal, parameter or wildcard — MUST be a
   plain (unquoted) identifier of ASCII letters, digits and
   underscores. Segment names flow verbatim into registration patterns
   and generated Go; the closed character set is what makes that safe
   (and is why `{`/`}` can serve as the collision-free conflict markers
   of §route-conflicts). The same constraint applies to the `resources` table
   name (§declaration), to the identifiers of pipeline names and plug
   references (§pipelines), to route handler names (§handler-references), and to import
   aliases and path segments (§imports).

### Routes

```ebnf
route = verb, route path, handler ref, [ settings ], newline ;
verb  = "get" | "post" | "put" | "patch" | "delete"
      | "options" | "head" | "any" ;
```

1. Routes appear only inside a Scope body. The verb is a contextual
   keyword, case-insensitive; `any` registers the route for every
   method (its registration carries no method).
2. A route's **full path** is the enclosing scopes' prefixes followed
   by its own path, in nesting order.
3. Registration semantics are `http.ServeMux` (Go ≥ 1.22) exactly:
   parameters register as `{name}`, wildcards as `{name...}`, and a
   root full path as `/{$}`. Matching is exact — Volt never registers
   subtree patterns.

### Handler references

```ebnf
handler ref = name, [ ".", name ] ;
```

1. A handler is `Controller.Action` — two exported Go identifiers, the
   controller neither an import qualifier nor the package's own name,
   so the action lives in the routes package — or a **query
   reference**, which makes the route a query route (§query-routes) with no
   controller at all: `pkg.Query`, whose first part is an import
   qualifier (§imports) and whose second names a generated query of that
   package, or `Query` or `<package name>.Query`, a generated query of
   the package itself (§query-routes), spelled as a plug is (§pipelines). The
   runtime's own handlers are `volt.<Name>` (§event-routes).
2. Every distinct controller becomes one generated interface; every
   distinct action one method with the route's typed parameters
   appended after `(w http.ResponseWriter, r *volt.Request)`, returning
   `error`.
3. Two routes MAY share `Controller.Action` only with identical
   parameter signatures (names, types and wildcard-ness, in order).
4. The generated `Controllers` manifest is one namespace: a controller
   MUST NOT take the name of a field the manifest holds for a data
   package (`DB`, `Queries`; §query-routes) or for the event broker
   (`Events`; §event-routes). The error names the field.

### Query routes

A **query route** binds a route to a generated query method of a data
package — an imported one, or the package itself when it declares the
tables — instead of to a controller. Volt generated the
method, so it knows the parameters and the row type, and it writes the
handler: bind, call, render. Nothing is dispatched by name at runtime.

```volt fragment
package app

import (
	db
)

Scope /api [pipe: api] {
	get    /users            db.UserList
	get    /users/:id(int) db.UserGet
	post   /users            db.UserCreate
	patch  /users/:id(int) db.UserUpdate
	delete /users/:id(int) db.UserDelete
	get    /picked           db.UserPicked      // Select picked for users where id in :ids
}
```

1. **Which methods qualify.** A select method, `<Model><SelectName>`
   per member (§selects-over-groups), or a default CRUD method of a table:
   `<Model>Get`, `<Model>List`, `<Model>Create`, `<Model>Update`,
   `<Model>Delete`, each only where the query generator emits it
   (`Get`/`Update`/`Delete` need a primary key, `Update` a non-key
   column). Any other name is an error listing what qualifies; a name
   differing only in case names the right spelling.
2. **Binding.** Each parameter of the method, in signature order,
   binds from exactly one source, decided at generation time:
   - a **path parameter** of the route with the same name binds it.
     Its declared type MUST spell the method's Go type
     (`:id(int)` for `id int`; `string` needs no annotation); a
     type a segment cannot carry (§route-paths) goes in the query string.
     A wildcard cannot bind a query parameter;
   - a **changeset** parameter (`<Model>Create`, `<Model>Update`)
     binds from the **request body**: the body decodes as
     `<Model>Params` by `Content-Type` (Formats; §changesets, `Params`)
     and is cast into a changeset. `Create` casts into `<Model>New()`
     allowing every column but the auto-increment key; `Update` loads
     the row by the key path parameters with `<Model>Get` (a miss is a
     404), casts into its `Changeset()` allowing every non-key column,
     and the route MUST carry every key column as a path parameter.
     The body is decoded before the row is loaded, so a malformed body
     is a 400 whether or not the row exists. A write route always takes
     a body, `{}` when nothing is set: a table whose only column is the
     auto-increment key still needs `{}`. The route's verb MUST be
     `post`, `put` or `patch`; `put` is accepted as a spelling of the
     same partial update as `patch`, the handler being the same cast,
     and nothing the body leaves out is reset;
   - a **list parameter** (§predicates) binds from the **repeated query
     key** of its name; absent means the empty list;
   - every other parameter binds from the **query string** by name,
     parsed to its type; a missing or unparseable value is a 400 that
     names the parameter.
   A parameter binds from the query string only when its type is
   `string`, `bool`, `int`, `float64` or `time.Time`, or a slice of one
   of those (a list parameter); a select with a parameter of any other
   type, a declared type (§types) included, is not a query route, and
   the route is an error. A path parameter the method does not take is
   an error: the route would capture a value nothing consumes; the key
   of an `Update` is consumed by the load.
3. **Response.** The result renders in the negotiated format
   (Formats): rows or a row with status 200, a created row with 201,
   a delete with 204 and no body. A query's row miss (`rt.ErrNotFound`,
   which is `sql.ErrNoRows`) is a 404 through the error spine. A
   refused write is a `*rt.Invalid` through the error spine: 409 for
   a unique violation, 422 otherwise, itemized per field (§changesets,
   `rt.Invalid`), an unknown key of the body among the fields. Any other
   error is the spine's 500.
4. **Names.** The helper name (§route-names-and-reverse-urls) defaults to the scope name
   prefixes followed by the method name, and only `get`/`head` query
   routes carry one — writes have no reverse URL, as with resources
   (§the-action-table). `volt routes` and the route table show the reference as
   written (`db.UserGet`; a query of the package itself unqualified,
   `UserGet`) in place of `Controller.Action`.
5. **Wiring.** The generated `Controllers` manifest (§handler-references) gains one
   field per data package that query routes go through, named by the
   qualifier as a Go name (`db` → `DB`), typed `*<pkg>.Queries` — for
   the package itself, `Queries *Queries`; the application
   constructs it with the package's `New`. No interface is generated
   for a query route.
6. **Own tables.** A package that declares tables MAY route them
   itself, so one directory holds schema, routes and handlers.
   `Query` or `<package name>.Query` names its own generated query;
   `resources t [default]` (§default-handlers) and `dataset s` (§dataset-declaration) resolve in
   the package when bare or self-qualified. Nothing is imported, the
   generated handlers name the package's changesets bare, and the
   router's constructor is `NewRouter`, so it never collides with the
   query layer's `New`. The package's own name is thereby a qualifier:
   an import MUST NOT take it (§imports).

### Formats

The wire format of a query route is negotiated by HTTP headers and
never spelled in the URL, so a path parameter can never be mistaken
for a format suffix.

1. **Responses** follow `Accept`: the first listed media type that is
   `application/json`, `application/x-gob`, `application/*` or `*/*`
   decides; an absent header means JSON; a header offering neither is
   a 406 naming both. Negotiation happens before the handler runs
   anything: a 406 decodes no body, loads no row and writes nothing.
2. **Request bodies** follow `Content-Type`: absent or
   `application/json` decodes JSON, `application/x-gob` decodes GOB,
   anything else is a 415; an empty or malformed body is a 400. A write
   body is a `<Model>Params` (§changesets): as JSON an object of the
   columns to set by wire name, as GOB the row and the marked columns;
   a key that names no column is a field error `unknown_field` of the
   changeset (422), every one of them reported, so a typo never
   vanishes silently. `volt.Decode` into a struct of a hand-written
   controller keeps the 400 whose body itemizes the first undeclared
   JSON field as `Detail{Check: "unknown_field", Columns: [<field>]}`;
   GOB ignores unknown fields there, the shared struct being the
   contract. A body over the runtime's `MaxBodyBytes` (four megabytes
   unless set) is a 413.
3. **GOB is the Go-native arm**: a client that imports the generated
   models package decodes rows into the same types the server
   encoded, with no schema on the wire. JSON is what browsers, curl and
   every other language see.
4. The runtime exposes the same rules to hand-written controllers as
   `volt.Negotiate`, `volt.Render`, `volt.RenderStatus` and
   `volt.Decode`, so a controller route can speak both formats without
   repeating them.
5. **Error bodies** are structured: the default error handler writes a
   `volt.Problem` — status, message, and details (check, columns,
   message) when the error itemizes them — in the negotiated format,
   JSON when the client offered nothing usable. A generated client
   decodes it into a `*volt.ProblemError`, so a caller reads
   `errors.As(err, &pe)` and marks the fields `pe.Details` name.

### Generated client

Every routing package also generates a **client package**,
`client/volt_client.go` beside it, so a Go program — the desktop
application that shares the schema, a test, another service — calls
the routes through typed methods instead of hand-built requests.

1. The package is named `client` and imports only the runtime and the
   data packages the routes go through — never the routing package —
   so a caller links no server code. The one exception is a package
   that routes its own tables (§query-routes): its client imports it for the
   row types, and so links it; when that package is `main`, which Go
   cannot import, no client is generated.
2. `type Client struct { volt.Client }` and `New(base string)
   *Client`. The embedded runtime client carries the origin, the
   `http.Client` and the wire format (§formats); a non-2xx reply comes
   back as an `HTTPError` with the status and body text, so
   `errors.Is(err, volt.ErrNotFound)` holds for a 404.
3. **Every query route** (§query-routes) is one typed method, named as its
   helper would be (scope name prefixes and the method name, or
   `name:`), with the query's parameters in signature order and the
   query's result: `func (c *Client) APIUserGet(ctx, id int)
   (db.User, error)`, `APIUserDelete(ctx, id int) error`. Path
   parameters render through the same builders as `Path*` helpers,
   query-string parameters through `volt.FormatQuery`, a list as a
   repeated key, a changeset as the body: `APIUserCreate(ctx, cs
   db.UserChangeset) (db.User, error)`, `APIUserUpdate(ctx, id int, cs
   db.UserChangeset) (db.User, error)`, sent as `cs.Params()`
   (§changesets). A changeset built from `<Model>New()` is the carrier
   of changes on the client side, so an update needs no row in hand:
   `APIUserUpdate(ctx, id, db.UserNew().EmailSet(e))` sets one column
   of row `id`.
4. **Every named controller route** is one raw method with the
   route's path parameters, returning `*http.Response` for the caller
   to read and close: Volt does not know what a hand-written action
   writes.
5. Client method names and reverse-URL helper names share one
   namespace per package (§route-names-and-reverse-urls); a collision is an error naming
   `name:`.
6. **Beside the models.** `volt gen -o DIR` (§projects-and-packages) writes the client
   as `DIR/volt_client.go` in the package `DIR` holds instead of as a
   subpackage, for a program that is the client — a desktop or browser
   front end that is `package main`. The row, changeset and params
   types are named bare, since the models written beside it declare them; the
   constructor is `NewClient(base string) *Client`, since `New` beside
   it may be the query layer's; nothing but the runtime is imported.

### Event routes

An **event route** streams server-sent events to every connected
client, so what one user does — starts a computation, finishes it,
changes rows — reaches every open screen without polling.

```volt fragment
Scope / [pipe: api] {
	get /events volt.Events
}
```

1. The handler `volt.Events` names the runtime's event stream; it is
   the only handler the runtime provides, and it takes `get`. The
   route otherwise behaves as any route: prefixes, pipelines, error
   handler, a helper and a client method named by the action
   (`PathEvents`, `client.Events`).
2. The `Controllers` manifest (§handler-references) gains `Events *volt.Broker`; the
   application publishes with `Events.Publish(name, data)`. The
   broker fans every event out to every open stream, assigns IDs in
   order, and keeps a replay buffer so a client that reconnects with
   `Last-Event-ID` resumes where it was. Events are
   `text/event-stream` with a JSON payload; a subscriber that falls
   behind is dropped and reconnects.
3. The generated client's `Events(ctx) <-chan volt.Event` connects,
   delivers every event, reconnects with backoff on any drop, and
   closes the channel when ctx is done.
4. Event names and payloads are the application's contract, not the
   language's: a `table.changed` event carrying keys, after which the
   client refetches what it shows, keeps one code path for loading.

### Scopes

```ebnf
scope      = "Scope", route path, [ settings ], "{", { scope item }, "}" ;
scope item = route | resources | dataset | scope ;
```

Scope settings (the complete set):

| Setting | Value | Meaning |
|---|---|---|
| `pipe` | Pipeline name | append the pipeline to the inherited chain |
| `name` | identifier | prefix for helper names beneath this scope |
| `error_handler` | function of this package (`Name` or `pkg.Name`) | error spine for routes beneath, nearest wins |

1. Scopes nest arbitrarily. Prefixes concatenate; `pipe` chains
   **append** (ancestors outermost); `name` prefixes concatenate;
   `error_handler` is overridden by the nearest enclosing setting.
2. A route's middleware is the concatenation of its pipeline chain's
   plugs, composed once at generation time — never iterated per
   request.
3. The `error_handler` function is a Go reference held like a plug
: it MUST exist in the package's compiled Go files with the
   volt runtime's ErrorHandler
   shape: `func(http.ResponseWriter, *volt.Request, error)`. Routes
   without one use the runtime default.

### Route names and reverse URLs

1. Every route derives a **helper name**: the scope name prefixes
   followed by the `[name:]` setting (normalized to a Go name) when
   present, else the action name. Resources derive per §table-resolution.
2. Each named route yields one generated function
   `Path<Helper>(typed params..., opts ...volt.URLOption) string` that
   produces exactly the paths its route matches. Helper names form one
   namespace per package; a collision is an error. Helper output is
   always a clean path: string parameters are percent-escaped, and the
   values `.` and `..` escape entirely (`%2E` forms) — as literal
   segments they would change the path's shape under cleaning.

### Route conflicts

1. A route's **shape** is its full path with every parameter replaced
   by `{}` and every wildcard by `{...}`; literals compare by spelling.
   (Braces cannot appear in a literal segment, §route-paths, so the markers
   cannot collide with any literal.)
2. Two routes **overlap** when some request matches both: their methods
   overlap (equal, or either is `any`) and their paths overlap (some
   request path satisfies both patterns). A route is **more specific**
   than another when every request it matches the other also matches,
   and not conversely. Two overlapping routes where **neither is more
   specific** are ambiguous — an error naming both source positions.
   Identical method-and-shape is the degenerate case (reported as a
   duplicate); routes differing in literal-vs-parameter at some
   position while one is strictly more specific (e.g. `/users/new` vs
   `/users/:id`) are legal, and ServeMux precedence picks the more
   specific one at runtime.
3. **Where an ambiguity can hide.** Call a route's *literal prefix*
   its segments before its first parameter, wildcard or end. Two
   routes overlap only if, at every position of the shorter fixed
   part, the segments are equal literals or at least one is a
   parameter, and any remaining segments of the longer are absorbed by
   the shorter's wildcard tail. Hence a route can be ambiguous with an
   accepted route only if the accepted route's literal prefix lies on
   the new route's walk: the same literal where the new route has one,
   any segment where it has a parameter, and anything at all beyond
   its length only when it ends in a wildcard. A checker MAY therefore
   compare a new route with the accepted routes filed under the literal
   prefixes on that walk alone; every other accepted route matches no
   request the new one does, and the earliest ambiguous route in
   declaration order is found by taking the earliest among the walk's.
4. This is exactly the rule `http.ServeMux` enforces by panicking at
   registration time. Detection therefore happens at check time, so a
   checked package always registers cleanly; the registration panic
   remains as a backstop a conforming generator never triggers.

```volt fragment
Scope / [pipe: api, error_handler: Errors] {
	get /            Home.Index [name: root]
	Scope /admin [name: admin] {
		get /stats   Admin.Stats
	}
	get /files/:path...           Files.Serve
	get /users/:id(int)/avatar  Users.Avatar
}
```

## Resources

```ebnf
resources  = "resources", table ref, [ settings ], newline ;
table ref  = name, [ ".", name ] ;
ident list = "(", name, { ",", name }, ")" ;   (* the value of only: and except:, a setting value (§settings-lists) *)
```

### Declaration

1. `resources <table>` appears only inside a Scope body and expands to
   the action routes of §the-action-table with the table name as the collection
   segment. The name MUST map to a Go identifier.
2. The declaration MUST resolve to a declared table (§table-resolution); there
   is no schemaless form. A miss — including a name that only matches
   a table's model name or differs only in case — is an error naming
   the correct spelling where one exists.

### The action table

| Action | Method(s) | Path | Helper |
|---|---|---|---|
| `index` | GET | `/<table>` | plural |
| `new` | GET | `/<table>/new` | `New` + singular |
| `create` | POST | `/<table>` | — |
| `show` | GET | `/<table>/:<param>` | singular |
| `edit` | GET | `/<table>/:<param>/edit` | `Edit` + singular |
| `update` | PATCH **and** PUT | `/<table>/:<param>` | — |
| `delete` | DELETE | `/<table>/:<param>` | — |

`update` expands to two routes sharing one action and signature.

### Settings

| Setting | Value | Meaning |
|---|---|---|
| `api` | flag | restrict to index, create, show, update, delete |
| `only` | action list `(index, show)` | keep only these actions |
| `except` | action list | drop these actions |
| `param` | identifier | key parameter name (default `id`) |
| `singular` | identifier | the singular used for member helper names (§table-resolution) |
| `default` | flag | generate the handlers from the table's default CRUD; implies `api` (§default-handlers) |

1. Action names in `only`/`except` are the lowercase names of §the-action-table;
   unknown names are errors. `only` and `except` MUST NOT be combined.
   `only`/`except` filter the action set after `api`.
2. The key parameter's type always comes from the resolved table's
   primary key (§table-resolution).
3. `singular` overrides the inflector of §table-resolution. It is required
   whenever singularization leaves the name unchanged, which would
   otherwise make the collection and member helpers collide.
4. The key parameter MUST NOT repeat a parameter of the enclosing
   scope (§route-paths): `Scope /farms/:id { resources plots }` is an
   error, reported once at the declaration, naming `param:` as the
   fix (`[param: plot_id]`). Under `[default]` the name is the
   generated CRUD's and cannot move, so the scope's parameter is the
   one to rename.

### Table resolution

1. The declaration names a **table** of this package (`posts`) or of an
   imported package (`db.posts`); a qualified reference marks the
   import used (§imports).
2. The name matches the table's declared name **exactly**: names are
   case-sensitive, and a name differing only in case is an error naming
   the right spelling. A name that matches nothing, qualified or not,
   is an error (§declaration).
3. The resolved table MUST have a single-column primary key of type
   `int` or `string`; that type becomes the key parameter's type.
   Composite, missing, or unroutable keys are errors.
4. A resolved declaration fixes every derived name from the schema:
   the URL segment and the controller come from the table name as
   written, and the member helper from the table's **model** name (the
   `[model:]` setting when present, else the singularized table name).
   With `[model:]` set, no singularization is guessed at all.
5. The `singular:` setting overrides the member-helper name for a
   table whose model name the inflector cannot separate from the
   plural (clause 6); `[model:]` on the table is the equivalent fix at
   the schema side, and wins when both apply.
6. Plural and singular MUST differ: a name whose singularization is the identity
   (`posty`, `data`, `series`) would give the collection and member
   helpers one name, and is an error naming `singular:` as the fix.
   Neither a route `name:` nor a scope `name:` can resolve it — the
   former is not a resources setting (§settings-whitelists), the latter prefixes both
   sides equally.

```volt fragment
Scope /api {
	resources db.users [only: (index, show, create)]
}
```

### Default handlers

```volt fragment
Scope /api {
	resources db.users [default]
	resources db.tags  [default, except: (delete)]
}
```

1. `[default]` turns every surviving action into a **query route**
   (§query-routes) over the table's default CRUD method — `index` → `List`,
   `show` → `Get`, `create` → `Create`, `update` → `Update`, `delete`
   → `Delete` — so the handlers are generated and there is no
   controller to write. The routes, their paths and their helpers are
   those of §the-action-table and §table-resolution; only the handler side changes. Rails and
   Phoenix scaffold such a controller once and hand it over; Volt
   regenerates it forever, and an action taken back with `except:` is
   written by hand as any route.
2. `default` implies `api`: `new` and `edit` are form pages, and a
   generated handler has nothing to return for them.
3. The table's CRUD is the data package's: an imported one for a
   qualified table (`db.users`), this package's own for a bare or
   self-qualified one (`users`, `app.users`; §query-routes), as for every
   query route. A bare table in a package that declares none is an
   error naming the qualified form.
4. The key parameter is named as the generated CRUD spells the
   primary-key column (`id` for `id`), because binding is by name
   (§query-routes); `param:` is an error with `default`. Type and
   routability follow §table-resolution unchanged.
5. An action whose CRUD method does not exist is an error naming the
   `except:` that drops it — `update` on a table whose every column is
   part of the key (there is nothing to set).
6. Naming: `index` and `show` keep their reverse-URL helpers and
   client methods (§table-resolution); `create`, `update` and `delete` have no
   reverse URL (writes never do, §query-routes) and a client method named
   `Create`/`Update`/`Delete` + singular, prefixed by the scope name
   like the form-page helpers `New`/`Edit` + singular would be.
   `update`'s PUT route shares the PATCH route's client method, and
   PUT is accepted as a spelling of the same partial update as PATCH:
   the handler is the same cast, and nothing the body leaves out is
   reset.
7. Statuses, binding, validation and rendering are §query-routes's: 201 for
   `create`, 204 for `delete`, 404 on a miss, the body cast into a
   changeset with the allow lists of §query-routes and validated by the
   write (§changesets), the body and the response in the negotiated
   format (§formats).

## Datasets

A **dataset** expands a group select of a data package into
one query route per member table, so browsing every table of a group
is one line, and the route and the query it calls are emitted from
the same iteration and cannot disagree.

```ebnf
dataset = "dataset", [ name, "." ], name, [ settings ], newline ;
```

```volt fragment
// db: Group da_farm = DA \ (da_kody, …)
//     Select browse for da_farm where rok = :rok and idgr in :idgr
Scope /da [pipe: api] {
	dataset db.browse [strip: 'da_', except: (da_r_r)]
	get /r_r Da.RR
}
```

### Dataset declaration

1. `dataset` appears only inside a Scope body and names a select: of
   an imported data package when qualified (`db.browse`), the
   qualifier marking the import used (§imports); of this package when
   bare or self-qualified (`browse`, `app.browse`; §query-routes). A name
   that is not a select of that package is an error, with a
   case-insensitive hint.
2. For every member of the select's target (§selects-over-groups), in member order,
   the dataset yields one `get` **query route** (§query-routes) whose path is
   the scope prefixes followed by one segment — the member's table
   name, with the `strip:` prefix removed — and whose handler is the
   member's select method (`<Model><SelectName>`). Every parameter of
   the select binds from the query string, a list parameter as a
   repeated key (§query-routes). The helper and client method names are the
   scope name prefixes followed by the method name (§query-routes).
3. Expanded routes obey every rule of routes: conflicts (§route-conflicts), one
   helper namespace (§route-names-and-reverse-urls, §generated-client). To hand-write one member's route,
   take it out with `except:` and declare it beside the dataset.

### Dataset settings

| Setting | Value | Meaning |
|---|---|---|
| `strip` | string | prefix removed from each member's table name to form its segment |
| `only` | table list `(da_a_a, da_b_b)` | keep only these members |
| `except` | table list | drop these members |

1. `strip:` MUST be a prefix of every kept member's name, and what
   remains MUST be a usable segment (§route-paths); otherwise an error
   names the member. Without `strip:` the segment is the table name.
2. Names in `only`/`except` are members of the select's target,
   spelled exactly; an unknown name is an error. `only` and `except`
   MUST NOT be combined.

## Settings whitelists

Settings valid on Volt elements, exhaustively (a setting not listed for
an element is an error on that element):

| Element | Settings |
|---|---|
| Scope | `pipe`, `name`, `error_handler` |
| route | `name` |
| resources | `api`, `only`, `except`, `param`, `singular`, `default` |
| dataset | `strip`, `only`, `except` |

The identifier-list value form `(a, b, c)` (production in Collected grammar (Part II))
is valid only where a setting explicitly takes an action list.

## Generation contract (informative)

The normative output contract is the golden corpus under
`gen/router/testdata/` and the proof suite under `itest/`. In prose:
one `<Controller>Controller` interface per controller and a
`Controllers` struct (§handler-references), with one `*<pkg>.Queries` field per data
package query routes use (§query-routes); `NewRouter(Controllers) http.Handler`
registering every route onto a `http.ServeMux` with its pipeline chain
composed statically and its typed shim parsing parameters per §route-paths
— for a query route the shim is the whole handler: it binds every
parameter per §query-routes, calls the query and renders per §formats;
`Path*` helpers per §route-names-and-reverse-urls; a `Table []volt.Route` mirroring the
expanded route list in declaration order, with `Query` set on query
routes; and the client package of §generated-client. For a data package that
declares migrations, `volt_migrations.go` carries them in statement form
(§migration-tooling). All generated files carry the
standard generated-code header and are gofmt-stable.

---

## Groups

A **Group** names a set of tables so the same code can be generated for
every member (§selects-over-groups). It is the one set construct of the
language: groups overlap freely, a table belonging to any number of
them.

```ebnf
group decl = "Group", plain name,
             ( "{", { newline }, [ group members ], "}"
             | "=", group expr ), newline ;
group members = plain name, { newline, { newline }, plain name }, { newline } ;
plain name    = plain identifier ;                      (* §identifiers *)
group expr    = group term, { ( "+" | "\" ), group term } ;
group term    = plain name
              | "(", plain name, { ",", plain name }, ")" ;
```

```volt fragment
Group series {
  ms_revenue
  ms_usage
  ks_seats
}

Group wide   = series + ks_costs \ ms_usage
Group public = Commerce \ (audit_log, api_keys)
```

1. Tables and groups share one namespace per package
   (§namespaces-and-uniqueness): a group MUST NOT take the name of a
   table, and redeclaring a group is an error.
2. A name in a `group term` resolves, case-sensitively, to a table or a
   group of the same package, tables first; the same order resolves a
   select's target (§selects-over-groups). An unknown name is an error
   with the same did-you-mean aids as §table-resolution.
3. The block form is the expression form with every member joined by
   `+`. Evaluation is left to right: `+` adds a term's member set
   (a table adds itself), `\` — set difference — removes it. A
   parenthesized term is the union of its names, applied one name at a
   time in the order written. Removing a table that is not currently a
   member, or adding one already present, is an error — the algebra
   must say something true. `-` is not a group operator: writing it is
   an error naming `\`.
4. Group references must be acyclic (error otherwise), and the
   resulting set must be non-empty.
5. Member order is first-addition order; generation (§selects-over-groups) is
   deterministic in it.

## Predicates

A **Pred** names a boolean expression over the columns of an as-yet
unnamed table. The expression language is deliberately **closed** —
it is not SQL and never grows toward it; anything it cannot say
belongs in the dynamic layer (§generated-query-surface).

```ebnf
pred decl    = "Pred", plain name, "{", { newline }, pred expr, { newline }, "}", newline ;
pred expr    = pred and, { "or", pred and } ;
pred and     = pred unary, { "and", pred unary } ;
pred unary   = [ "not" ], pred primary ;
pred primary = "(", pred expr, ")"
             | comparison | membership | pattern | null test
             | plain name - pred keyword ;       (* reference to a Pred *)
comparison   = operand, comp op, operand ;
comp op      = "=" | "!=" | "<" | "<=" | ">" | ">=" ;
membership   = column ref, "in", ( "(", literal, { ",", literal }, ")" | param ) ;
pattern      = column ref, "like", ( string | param ) ;
null test    = column ref, "is", [ "not" ], "null" ;
operand      = column ref | param | literal ;
column ref   = name - pred keyword ;
param        = ":", plain name ;                 (* no space after ':' *)
literal      = number | string | boolean ;
pred keyword = "and" | "or" | "not" | "in" | "like" | "is" | "null" ;
```

```volt fragment
Pred current { org = :org and year = :year }
Pred recent  { year >= :since }
Pred fresh   { current and recent }
Pred chosen  { org in :orgs }
```

1. `and`, `or`, `not`, `in`, `like`, `is`, `null` are keywords inside a
   predicate expression, case-insensitive (§keywords): a column of one of
   these names is referenced quoted (`"in" = 1`). Outside predicates
   they are ordinary identifiers.
2. A bare name in primary position references a Pred of the same
   package; references must exist and be acyclic.
3. A Pred is typed **at each use site** (§selects-over-groups), where a target binds
   its column names. The rules:
   - a `column ref` must resolve in the target's environment (§selects-over-groups);
   - `=` and `!=` require both operands to share a type class;
   - `<` `<=` `>` `>=` require numeric or date/time operands — text is
     **not** orderable here (`text_column > 1` and
     `text_column > 'a'` are both errors; order text in SQL, §selects-over-groups);
   - `like` requires a text column and a text pattern;
   - `in` items must all match the column's type;
   - a numeric literal MUST be representable in the column's storage
     type (§columns): against `int` it is an integer literal that fits
     int64, against `float64` a finite value; `9223372036854775808`,
     `1e400`, and `1.5` or `1e2` against `int` are errors. The generated
     method binds the literal in canonical Go form, so `-010`, `007`
     and `1E2` bind `-10`, `7` and `100.0`;
   - `in :name` makes `:name` a **list parameter**: its Go type is a
     slice of the column's generated type (`[]string`, `[]int`),
     and the column MUST NOT be `time.Time` — JSON, the list's
     wire form (§selects-over-groups), carries no date/time value;
   - `[]byte` columns may not appear in predicates;
   - a `param` adopts the type of the expression position it appears
     in; one param name MUST resolve to one type across the whole use
     site — a list and a scalar use of the same name disagree — or
     the use is an error naming both positions.
4. `[]byte` (rule 3) is the one column type predicates refuse: every
   other column, a column of a declared type among them, compares
   under rule 5.
5. Type classes follow the column types (§columns): numeric = `int`
   and `float64`; text = `string`; date/time = `time.Time`; `bool`
   agrees with `bool` only. A column of a declared type (§types)
   compares with `=`, `!=`, `in` and `is null` only: a parameter in
   that position has the declared Go type, a literal has the class
   of the storage type, and a declared type stored as `time.Time` or
   `[]byte` takes no literal. A list parameter of a declared type is
   bound as one JSON array of its values, by kind for a kinded type
   and through the type's driver value otherwise (§types). A declared
   type stored as `time.Time` cannot take one, as rule 3 says of
   `time.Time`, and neither can one stored as `[]byte`: JSON carries
   no blob, and the text its bytes would travel as never equals the
   stored value.
6. **Rendering.** There is one SQL renderer, the runtime's
   (`rt.SelectRender` and its siblings), and every statement Volt
   emits is its output: the dynamic layer calls it at run time, the
   generator calls it at generation time over the same tree and
   writes the result into the statement constants, so the same
   predicate spells the same bytes on both paths. Its spellings are
   fixed and it produces no other:
   - an identifier is always double-quoted (`"email"`), a `"` inside
     it doubled;
   - a comparison is `"col" <op> ?` or `"col" <op> "other"` with
     `<op>` one of `=`, `!=`, `<`, `<=`, `>`, `>=`; `!=` is the one
     spelling of inequality, `<>` is never produced and the dynamic
     layer has no operator for it;
   - membership in values is `"col" IN (?, ?)`, `NOT IN` likewise; an
     empty list is the constant `1 = 0` (`1 = 1` negated); a list
     parameter is `"col" IN (SELECT value FROM json_each(?))`, bound
     as one JSON array (`rt.JSONList`);
   - a pattern is `"col" LIKE ?`, `"col" NOT LIKE ?`; a null test
     `"col" IS NULL`, `"col" IS NOT NULL`;
   - `and` and `or` of two or more operands are parenthesized,
     `(a AND b AND c)`; one operand is itself; `not` is `NOT (x)`, and
     when x brings its own parentheses (an `and`/`or` group, a raw
     fragment) they serve: `NOT ("a" = ? OR "b" = ?)`;
   - a value is always an argument behind a positional `?`, in
     left-to-right order of the text: a literal written in a Volt
     predicate is bound by the generated method like a parameter, the
     statement text carries no literal (the constants `1 = 0`, `1 = 1`
     and `-1` of `LIMIT -1 OFFSET ?` are structure, not values);
   - an ordering is `ORDER BY "col" ASC, "other" DESC`, the direction
     always spelled;
   - a WHERE clause is the statement's predicates joined by ` AND `,
     each spelled as above, and is omitted when none survives;
   - the keyset position `After(k1, …, kn)` over `ORDER BY t1, …, tn`
     is one more predicate of the WHERE clause, `t1 > k1 or (t1 = k1
     and t2 > k2) or …` with `<` for a DESC term, spelled by the rules
     above: `("t1" > ? OR ("t1" = ? AND "t2" > ?))`, and a single term
     `"t1" > ?`;
   - a select is `SELECT [DISTINCT ]<columns> FROM <table>[ WHERE
     <predicate>][ ORDER BY <terms>][ LIMIT ?[ OFFSET ?]]`, the column
     list and the table the pre-rendered constants of the model; an
     offset without a limit is `LIMIT -1 OFFSET ?`;
   - a count is `SELECT count(*) FROM <table>[ WHERE <predicate>]`, an
     existence test `SELECT EXISTS (SELECT 1 FROM <table>[ WHERE
     <predicate>])`;
   - a delete is `DELETE FROM <table> WHERE <predicate>` and a partial
     update `UPDATE <table> SET "c" = ?, … WHERE <predicate>`, the
     assignments' values bound before the predicate's; neither renders
     without an effective predicate;
   - a changeset's insert (§changesets) is `INSERT INTO <table> ("c", …)
     VALUES (?, …) RETURNING <columns>`, or `INSERT INTO <table> DEFAULT
     VALUES RETURNING <columns>` when no column is marked, and its
     update `UPDATE <table> SET "c" = ?, … WHERE "k" = ?[ AND "k2" = ?]
     RETURNING <columns>`, the key columns bound after the assignments.
   A test renders a corpus of predicates once through the typed
   handles and once from Volt source and compares the bytes; the
   generator goldens and the integration tests prepare the constants
   on a real SQLite.

## Selects over groups

A **Select** declares a query once and generates it for every member
of a target: a group (§groups), or a single table treated as a
one-member group.

```ebnf
select decl = "Select", plain name, [ projection ], "for", plain name,
              [ "where", pred expr ], [ settings ], newline ;

projection  = "(", column name, { ",", column name }, ")"
            | "(", "*", "\", column term, { "\", column term }, ")" ;
column term = column name
            | "(", column name, { ",", column name }, ")" ;
column name = name ;
order list  = "(", order term, { ",", order term }, ")" ;   (* the value of order:, a setting value (§settings-lists) *)
order term  = column name, ( "asc" | "desc" ) ;
```

```volt fragment
Select rows    for series where current [order: (year desc, id asc)]
Select stale   for ms_usage where not recent
Select all     for series
Select summary (id, org, year)    for series where current
Select public  (* \ password_hash) for accounts
Select brief   (* \ (password_hash, bio, avatar)) for accounts
```

The clauses read in SQL order — name, columns, source, filter — and
the optional projection narrows the emitted columns.

1. The select name is a plain identifier; per target-member it mints
   the method `<Model><SelectName>` (§mapping-to-go naming). A collision
   with a generated CRUD/dynamic method name, or between two selects
   on an overlapping member, is an error.
2. `for` names a table or a group of the package, resolved as a group
   term is (§groups): case-sensitively, tables first, with the same
   did-you-mean aids. Since a group cannot take a table's name, at most
   one declaration matches.
3. `where` takes a full §predicates expression — named Preds, inline
   comparisons, and any composition of the two. Omitted = all rows.
4. **The agreement rule.** Every column referenced by the `where`
   expression (Pred references expanded) and by `order:` MUST exist in
   **every** member, and all members MUST agree on its type (§predicates).
   A missing column or a type disagreement is an error naming the
   offending members and their types — the predicate must be checkable
   for all members or it does not compile.
5. Settings: `order:` takes a parenthesized list of `column asc` /
   `column desc` pairs. The direction is **mandatory**: SQL's silent
   `asc` default is deliberately not inherited, explicit over implicit,
   and the emitted SQL spells it out (`ORDER BY "year" DESC, "id" ASC`).
   Order columns obey the agreement rule and MUST be orderable (§predicates) or text —
   ORDER BY text is SQL's own collation, allowed here — and MUST appear
   at most once. A column of a declared type (§types) is neither: it
   compares for equality only, so ordering it is an error.
6. Generation contract (with §mapping-to-go): per member, a method on
   `Queries` —
   `func (q *Queries) <Model><SelectName>(ctx context.Context, <params>) ([]<Row>, error)`
   — parameters in first-appearance order over the expanded
   expression, each typed by §predicates; `<Row>` is the member's row
   type (the model itself when there is no projection); the
   emitted SQL is the runtime renderer's output (§predicates,
   rendering), `SELECT <projected columns> FROM <table> [WHERE …] [ORDER BY
   …]` with positional `?` placeholders, the method binding its
   parameters and the expression's literals in the order the text
   names them. A list parameter renders as `<column> IN (SELECT value
   FROM json_each(?))` and binds one JSON array (`rt.JSONList`), so
   the statement is the same text for every list length and an empty
   list matches no row. The
   rendering is proven by
   construction (typed operators over quoted identifiers) and by the
   generator's corpus, whose select statements are prepared against
   the generated DDL on a real SQLite and executed through a real
   driver in `nao/itest`.
7. **Projection and row types.** Without a projection the row type is
   the member's model and the emitted columns are all of it. With one:
   - **Explicit list** `(a, b, c)` — at least one column, no
     duplicates. Every listed column MUST exist in every member with
     one agreed generated **field type** (§mapping-to-go — nullability
     included, so `string` and `string [null]` disagree); a miss or a
     disagreement is an error naming the members and their types. The
     row type is **one shared struct** named by the select name
     mapped per §names (select `summary` → `Summary`), serving
     every member: one wire type, N sources. Its fields keep the
     columns' declared order from the list, each with the agreed type
     and the **default** tag pair `db:"col" json:"col"` (§struct-tags): the
     shared type belongs to the select, not to any one table, so
     per-table `tag:` passthroughs and doc comments do not transfer.
     Agreement is judged on the generated field type, so a column of a
     declared type agrees exactly when every member types it with the
     same declared type.
   - **Star with exclusions** `(* \ a \ b)`, or as a set
     `(* \ (a, b))`, with the same `\` and set term as group algebra
     (§groups); `-` is an error naming `\`. At least one exclusion,
     no duplicates. Every excluded column MUST exist in every member
     (§groups: the algebra must say something true), and no member may
     end up with zero columns. Each member projects its own columns,
     in declaration order, minus the exclusions, minting a per-member
     **struct derivative** named `<Model><SelectName>`
     (`User` + `public` → `UserPublic`): every kept field is copied
     verbatim from the model — Go name, Go type, assembled struct tag
     (§struct-tags, `tag:` passthroughs included), doc comment, so the
     derivative behaves exactly like the model minus the removed
     fields.
   - Columns referenced by `where` or `order:` need not be projected.
   - Minted row-type names (the shared struct, and each derivative)
     live in the generated package scope and MUST NOT collide with any
     generated package-level name. The checker reports collisions with
     models, the package's bare declared types (§types), the `Queries`
     handle, another select's minted row type, the generated changeset
     names (`Changeset`, `Column`,
     `Params` and `New` select names are rejected outright), and the
     member tables' own column handles; the Go compiler backstops the rest.

---

## Validators

A **validator** is a Go function of the data package that takes the
values of some columns of a table and returns an error. The schema
names which validators guard which columns; the generator calls them
from the generated `Validate` methods; the functions remain ordinary
Go that any code may call. Nothing about a validator reaches the
database.

```ebnf
validators block = "validators", "{", { validator }, "}" ;
validator        = go ref, "(", column name, { ",", column name }, ")", newline ;
```

`validators block` is an alternative of `table body` and of
`table partial body` (§table, §tablepartial); a body MAY contain more
than one, and their lines accumulate, injected partials' first
(§partial-injection-order).

```volt
Table users {
  email string [required]
  hits  int

  validators {
    EmailShape(email)
    HitsSane(hits)
    Consistent(email, hits)
  }
}
```

```go
func EmailShape(email string) error {
	if !strings.Contains(email, "@") {
		return errors.New("needs an @")
	}
	return nil
}
```

1. **Resolution.** `go ref` names a function of the **containing
   package**, bare (`EmailShape`) or qualified by the package's own
   name (`db.EmailShape`); an imported package's function is an error
   suggesting a local wrapper. The function MUST exist as an exported,
   non-generic, top-level function in the package directory's non-test
   Go files that the go tool would compile for it (go/build's file
   constraints, as §pipelines); a file carrying the generated marker
   is invisible. A lower-case name is an error before resolution is
   attempted: a validator is part of the package's API.
2. **Signature.** The contract is `func <Name>(<the columns' Go
   types>) error`, and it is checked: exactly one parameter per listed
   column, in order, each spelled exactly as the column's generated Go
   type (§mapping-to-go: `string`, `int`, `time.Time`, `[]byte`,
   `rt.Null[T]` for a nullable column, a declared type's own name), a
   variadic parameter never matching, and exactly
   one result, `error`. Each violation is its own error, undeclared,
   unexported, generic, variadic, wrong arity, a parameter whose
   spelled type disagrees with the column, a wrong result, and every
   message names the exact signature to write, so an unimplemented
   reference reads as a to-do. A Go file with a syntax error hides the
   declarations after the error; the undeclared message then names the
   file and position. The Go compiler remains the authority on what a
   spelled type denotes.
3. **Columns.** Each `column name` MUST be a column of the enclosing
   table after partial injection. A column MAY appear in any number of
   validators and MAY be listed more than once in one.
4. **Order.** Validators run top-down, in declaration order, every one
   of them, every time; `required` runs first. Every failure is
   collected; the first does not stop the rest.
5. **Pure.** A validator receives values and returns an error. The
   generator never calls it except from `Validate`, and it receives
   no database handle, no context and no row. A rule that needs the
   database is not a validator: uniqueness and referential integrity
   are the unique index and the foreign key, which only the database
   can close against concurrent writers, and they reach the caller as
   `*rt.Invalid` (§changesets).
6. **Generation contract.** `Validate` is the changeset's (§changesets):
   `nao_validate.go` declares `func (c <Model>Changeset) Validate()
   <Model>Changeset` for every table, which runs rule 4 over `Row()`
   and records each failed validator as a field error whose check is
   the validator's name, whose columns are the ones it reads and whose
   message is the returned error's. Nothing else calls a validator;
   `Create` and `Update` call `Validate` before writing.
7. **Statuses and attribution.** A refused write is one `*rt.Invalid`,
   422 for failed validators and 409 for a unique violation, itemizing
   every field error (§changesets), which the error spine renders into
   the error body (§formats) so a form can mark the fields.
8. **`required` columns** (§columns) carry the one built-in rule of
   `Validate`, which the schema switches on per column. The type's
   empty value is refused, Go only, like every validator. Membership
   in a value set is a validator like any other, written beside the
   type (§types).
9. **No database twin.** A validator produces no DDL. The database
   never carries a `CHECK`: a column of a declared type is its storage
   type, a `required` column is `NOT NULL` and nothing more
   (§mapping-to-sqlite-ddl).

---

## Changesets

A **changeset** is a generated value that carries a row, the changes
proposed to it, and the errors found so far. It is built by pure
functions, validated by pure functions, and written by one call at
the end. Nothing happens to the database before that call, and
nothing happens if the changeset is invalid. Every generated write
takes a changeset: there is no other write path for a row.

```go
type UserChangeset struct { /* unexported: data, changes, marks, errors */ }
type UserColumn = rt.ColumnOf[User]   // the column handles UserID, UserEmail, ...
type UserParams = rt.Params[User]     // a write body on the wire

func UserNew() UserChangeset                                     // for a create
func (u User) Changeset() UserChangeset                          // for an update
func (c UserChangeset) From(u User) UserChangeset                // every data column of u

func (c UserChangeset) EmailSet(v string) UserChangeset          // one per column
func (c UserChangeset) BioSet(v rt.Null[string]) UserChangeset
func (c UserChangeset) Cast(p UserParams, allow ...UserColumn) UserChangeset

func (c UserChangeset) Changed(col UserColumn) bool
func (c UserChangeset) Row() User                                // data with changes applied
func (c UserChangeset) Params() UserParams                       // the changes, for the wire
func (c UserChangeset) Validate() UserChangeset                  // required, validators
func (c UserChangeset) ErrorAdd(col UserColumn, msg string) UserChangeset
func (c UserChangeset) Errors() []rt.FieldError
func (c UserChangeset) Valid() bool
func (c UserChangeset) Err() error                               // *rt.Invalid, nil when valid

func (q *Queries) UserCreate(ctx context.Context, c UserChangeset) (User, error)
func (q *Queries) UserUpdate(ctx context.Context, c UserChangeset) (User, error)
```

```go
cs := user.Changeset().Cast(params, db.UserEmail, db.UserBio).Validate()
if !cs.Valid() {
	return cs.Err()
}
user, err := q.UserUpdate(ctx, cs)
```

1. **Value semantics.** A changeset is a struct passed and returned
   by value; every method returns a new changeset and leaves its
   receiver as it was, so two goroutines may hold copies of one
   changeset with no coordination. No method of a changeset touches
   the database.
2. **Data and changes.** `UserNew()` starts from the zero row with the schema's literal
   defaults in place (a number, string or boolean `default:` of a
   built-in column or of a kinded declared type, §default-values,
   §types) and nothing set, so `Row()` and `Validate` see the literal
   default the database writes for an unmarked column that has one,
   and NULL for an unmarked column that carries `null` with no
   default; an expression default, and the default of a declared type
   that implements Scanner and Valuer, are the database's alone and
   read as the zero value until the row is stored, as the
   auto-increment key does until the database assigns it. An unmarked NOT NULL column without a default
   reads as its zero value too, but the insert names no value for it,
   so the database writes NULL and refuses the row as a `not_null`
   field error (rule 8), unless `Validate` refused that zero value
   first. `u.Changeset()` starts from `u` with nothing set and
   remembers that it was made from a row. A setter records a value
   for one column and **marks** it; `Changed` reports the mark; a
   column set to the value it already holds is still a change, since
   the changeset records intent, not difference. `Row()` is the data
   with the marked values applied. `From(u)` marks every data column
   of `u` but the auto-increment key. The auto-increment key has no
   setter: the database assigns it.
3. **Setters.** One per column, named `<Field>Set` after the field's
   Go name (§names), taking the field's Go type: `rt.Null[T]` for a
   nullable column, so `BioSet(rt.Null[string]{})` sets NULL. The
   fixed methods of a changeset never end in `Set`, so a setter cannot
   collide with one; a column whose Go field would be `Changeset` is
   an error, since the row's `Changeset` method occupies that name.
4. **`Cast`.** For every column in `allow` that `p` carries, converts
   the carried value to the column's Go type and sets it; a value
   that does not convert is a field error with check `cast` on the
   column whose message names the key as sent and the column's type
   as the schema spells it (`displayName: expected string`), `null`
   on a non-nullable column among them likewise (`email: null is not
   allowed (string)`), and the column is left as it was. Columns `p`
   does not carry are untouched, which is what makes a `PATCH` body
   partial. A column of `p` outside `allow` is ignored, never an
   error: the schema and the handler decide what a client may write,
   the client cannot widen it. A key of `p` that names no column of
   the table is a field error with check `unknown_field`, so a typo
   never vanishes.
5. **`Params`.** `rt.Params[M]` is a write body as the wire carries
   it, in either format (§formats): as JSON, an object of the changed
   columns keyed by their wire names (the `json` tag, §struct-tags),
   each value as `encoding/json` renders it and `null` for NULL, a
   marked column always among them whatever else its tag says; as
   GOB, the row and the list of marked columns, typed by the shared
   model. `c.Params()` builds one from the marks of `c`, each an
   `rt.Mark` of wire name and value; `volt.Decode` reads one from a
   request; `Cast` consumes one. A body keeps its arm through a GOB
   re-encoding: one read from JSON carries its JSON object, so `Cast`
   reads it the same on either side, and one built from marks carries
   their JSON too. A JSON body that is not an object is a malformed
   body (400).
6. **`Validate`.** Runs over `Row()`, in this order, every rule every
   time: each `required` column (rule 9), then every validator of the
   table top-down (§validators). Every failure is collected as a field error; the
   first does not stop the rest. `Validate` is pure and idempotent;
   `Create` and `Update` call it again, so forgetting it costs only an
   earlier error message.
7. **Errors.** `ErrorAdd` records a field error with check `custom`
   on a column, for a rule the controller alone can judge. `Errors`
   lists the `cast`, `unknown_field` and `custom` errors in the order
   recorded, then the findings of the last `Validate`; `Valid` is "no
   field errors"; `Err` is `nil` when valid and otherwise a
   `*rt.Invalid` carrying them. `rt.FieldError` is `{Check, Columns,
   Message}`: the check is `required`, `cast`,
   `unknown_field`, `custom`, a validator's name, or the database's
   `unique`, `foreign_key`, `not_null` (rule 10); the columns are the
   ones the check reads.
8. **`Create` and `Update`.** `<Model>Create(ctx, c)` validates and,
   if valid, inserts the marked columns and returns the stored row;
   a column never marked takes its database default or, lacking one,
   NULL, which a `NOT NULL` column refuses as a `not_null` field error
   (rule 10); nothing marked inserts `DEFAULT VALUES`.
   `<Model>Update(ctx, c)` validates and, if valid,
   issues `UPDATE <table> SET <the marked columns> WHERE <the key of
   the row the changeset was made from> RETURNING` and returns the
   stored row; nothing marked touches nothing and returns `Row()`;
   a changeset not made from a row (`UserNew()`) is `rt.ErrNoRow`,
   there being no key; a vanished row is `rt.ErrNotFound`. Both
   statements are rendered at run time from the marks and run through
   `rt.StmtCache` when `WithCache` was set, like the dynamic layer.
   An invalid changeset is `*rt.Invalid` from either, before any
   statement runs.
9. **`required`** (§columns) is the one built-in rule of `Validate`
   (rule 6), which the schema switches on per column: it refuses the
   type's empty value, `""` for `string`, `0` for `int` and
   `float64`, a zero length for `[]byte`, and for a declared type the
   empty value of its storage type, compared directly for a kinded
   type and read through the type's `driver.Valuer` otherwise
   (§types). It is Go only, like every validator.
10. **`rt.Invalid`.** Every refused write is one `*rt.Invalid{Model,
    Fields, Cause}`: the field errors of the changeset when validation
    failed, or the database's refusal mapped onto fields when the
    statement ran: a unique violation becomes a field error with
    check `unique` on the index's columns, a NOT NULL violation
    `not_null` on its column, a foreign key violation `foreign_key`
    with no column (SQLite names none), each wrapping the driver's
    error as `Cause`. Its status is 409 when a `unique` field error is
    among them, 422 otherwise; it itemizes itself (`Details()`: check,
    columns, message), which the error spine renders into the error
    body (§formats) so a form can mark the fields. Any other database
    error is returned as is.
11. **Bulk paths bypass.** `UpdateWhere` and `DeleteWhere` of the
    dynamic layer take assignments and predicates, never a row, and
    run no validator; `Delete` runs none either. They are the
    equivalent of raw SQL and are documented as such.
12. **Generation contract.** `nao_models.go` declares per model the
    changeset type, `<Model>New`, `Changeset`, `From`, the setters,
    `Cast`, `Changed`, `Row`, `Params`, `ErrorAdd`, `Errors`, `Valid`,
    `Err`, the aliases `<Model>Column` and `<Model>Params`, and the
    column handles `<Model><Field>` (`rt.Column[M, T]`,
    `rt.NullColumn` when nullable), so a program that shares the
    models, a generated client among them, builds changesets with
    nothing else. `nao_validate.go` declares `Validate`, which is
    where the package's validator functions are called.
    `nao_queries.go` declares `Create` and `Update`. The names
    `<Model>Changeset`, `<Model>Column`, `<Model>Params` and
    `<Model>New` join the package-scope names the `dynname` lint and
    §selects-over-groups guard.

---

## Migrations

A **migration** is a named list of structural operations, written by
hand or by a tool over the drift report (§the-fold). Migrations are
separate from the schema: the schema says what the database is, a
migration says how one database becomes the next. A migration is not
checked against the schema operation by operation, since it may say
anything that was once true; the composition of all of them is
(§the-fold).

```ebnf
migration          = "Migration", migration name, [ migration settings ],
                     "{", migration body, "}" ;
migration name     = digit, digit, digit, digit, { digit }, "__", label ;
label              = ( letter | digit ), { letter | digit } ;

migration settings = "[", migration setting, { ",", migration setting }, "]" ;
migration setting  = "irreversible"
                   | "no_transaction" ;

migration body     = { operation }
                   | "up", "{", { operation }, "}",
                     [ "down", "{", { operation }, "}" ] ;

operation          = create table op | drop table op | rename table op
                   | alter table op  | rebuild table op
                   | create index op | drop index op
                   | exec op ;

create table op    = "create", "table", name, "{", shape body, "}" ;
drop table op      = "drop", "table", name, [ "{", shape body, "}" ], newline ;
rename table op    = "rename", "table", name, "to", name, newline ;

alter table op     = "alter", "table", name, "{", { alter item }, "}" ;
alter item         = "add", migration column
                   | "drop", name,
                     [ column type, [ migration column settings ] ], newline
                   | "rename", name, "to", name, newline ;

rebuild table op   = "rebuild", "table", name, "{", shape body, "}" ;

create index op    = "create", "index", name, index key,
                     [ index settings ], newline ;
drop index op      = "drop", "index", name, index key,
                     [ index settings ], newline ;

exec op            = "exec", string, [ exec settings ], newline ;
exec settings      = "[", "down", ":", string, "]" ;

shape body         = { migration column | indexes block } ;

migration column          = name, column type,
                            [ migration column settings ], newline ;
migration column settings = "[", migration column setting,
                            { ",", migration column setting }, "]" ;
migration column setting  = column setting
                          | "from", ":", expression literal
                          | "delete", ":", ref action
                          | "update", ":", ref action ;
```

`column type`, `column setting`, `index key`, `index settings` and
`indexes block` are the productions of the schema core
(§columns, §indexes) and `ref action` is that of §relationships-ref,
all narrowed by the rules below; a migration column adds `from:`
(rule 9) and the referential actions `delete:` and `update:` (rule
4) to the column settings, since the schema declares the actions on
a `Ref` and a migration has no `Ref`. The
words of a migration body (`up`, `down`, `create`, `drop`, `rename`,
`alter`, `rebuild`, `exec`, `table`, `index`, `add`, `to`) are
keywords inside a `Migration` only; elsewhere they are identifiers.

```volt fragment
Migration 0002__users_bio {
  alter table users {
    add bio string [null]
    rename name to full_name
  }
  create index users (full_name)
}
```

1. **Placement.** A migration is an element of the data package it
   migrates: the package that declares the tables. It may stand in
   any `.volt` file of that package, alone, among other migrations,
   or beside the schema. No file name and no directory is prescribed.
   A package that declares no tables MUST NOT declare migrations.
   Migrations are never imported (§imports).
2. **Name and order.** A migration name is a run of at least four
   digits, two underscores and a label; it is a plain identifier
   (§identifiers) and MUST be unique within the package. Every
   migration name of a package MUST have the same digit-run width; a
   package mixing widths is an error naming both names. Migrations
   apply in byte order of their names, which is numeric order of the
   run since the runs share a width. The name is the whole order; the
   file and the position within it carry no meaning
   (§projects-and-packages). `volt migration new LABEL` mints the
   next name at the package's width: one more than the greatest run
   among the package's migrations, zero-padded to that width (four
   digits in a package without migrations), then `__`, then the
   label; when the next number does not fit the width it is an error
   telling the user to widen every name.
3. **Self-contained.** A migration references nothing declared
   elsewhere. Table, column and index names are plain names, not
   resolved against the schema. A column type MUST be one of the six
   built-in types (§columns); a declared type is not a
   type here, since the database holds its storage type. A migration
   column is NOT NULL unless it carries `null`, as a schema column is,
   a key column MUST NOT carry `null`, and a `shape body` has at most
   one primary key, one `pk` column or one `[pk]` index, as a table
   has (§columns). A partial injection, a `validators` block and a
   records block are not part of a `shape body`.
4. **Settings whitelist.** In a migration, a column setting is one of
   `pk`, `increment`, `null`, `unique`, `default:`, `ref:`, `delete:`,
   `update:`, and in a `rebuild` also `from:` (rule 9);
   `from:` anywhere else is an error.
   `ref: table.column` is spelled as in the schema (§columns): the
   declaring column holds the key. `delete:` and `update:` take the referential actions of
   §relationships-ref (`cascade`, `restrict`, `set_null`,
   `set_default`) for the column's `ref:`, so that a schema
   foreign key with an action can be matched; without a `ref:` they
   are errors. As there, `set_null` requires the column to carry
   `null`, and `set_default` requires `null` or a `default:`. An index
   setting is one of `unique`, `pk` (in a shape body only, rule 10),
   `name:`, `where:`; on a `pk` index `name:`, `where:` and an
   expression atom are errors, as in §indexes. `tag:`, `required` and
   `model:` describe Go, and `note:` documents the schema, which
   generated code carries; all four are errors here, in every position
   of a migration. A migration takes `irreversible` and
   `no_transaction` and no other setting; an `exec` takes `down:` and
   no other setting.
5. **`create table`.** The body MUST contain at least one column;
   names are unique within it. It lowers by the DDL mapping exactly
   as the schema's `Table` does (§mapping-to-sqlite-ddl), index names
   included.
6. **`drop table`.** Drops the table and, with it, its indexes. The
   optional body is the shape being dropped and exists only so that
   the migration is reversible (rule 12); when written, it MUST
   contain at least one column, as a `create table` body must.
7. **`rename table`.** `ALTER TABLE … RENAME TO`. Foreign keys that
   reference the table follow it; index names do not, and are kept
   as they are.
8. **`alter table`.** Each item is one `ALTER TABLE` statement, in
   the written order. `add` is `ADD COLUMN` under SQLite's rules,
   checked here rather than at run time: a column without `null` MUST
   carry a `default:`; the default MUST be a literal; `pk` and
   `increment` are errors; `ref:` requires the column to carry `null`;
   a column with `ref:` takes no `default:`, since SQLite refuses to
   add a `REFERENCES` column with a default other than NULL to a table
   that has rows; `unique` lowers to the added column plus
   a `CREATE UNIQUE INDEX` on it. `drop` is `DROP COLUMN`; the
   optional type and settings are the dropped column's shape, which
   in a flat body is the inverse `add` and MUST satisfy the `add`
   rules above (rule 12), and inside an `up { }` or `down { }` block
   is checked as a column only, since there the inverse is written
   out. `rename` is `RENAME COLUMN`.
9. **`rebuild table`.** The one operation for every change SQLite
   cannot `ALTER`: a type, a nullability, a default, a primary key, a
   foreign key. The body is the new shape and MUST contain at least
   one column. It lowers to: create `<table>__new` from the body;
   `INSERT INTO <table>__new (<cols>) SELECT <sources> FROM <table>`;
   `DROP TABLE <table>`; `ALTER TABLE <table>__new RENAME TO
   <table>`; create every index of the body. The source of a column
   is, in order: its `from:` expression literal if given; the old
   column of the same name; its `default:`; `NULL`. A column without
   `null` whose source is `NULL` is an error, except a rowid alias
   (`int [pk, increment]`) introduced by the rebuild, which takes
   `NULL` as its source and the database assigns it. The source order
   is checked here for a table the package's migrations created; a
   table of unknown shape takes the old column of the same name when
   the fold has it and otherwise errors at fold time (§the-fold).
   Nothing but the body survives the rebuild: an index not restated
   in it is gone. A rebuild of a table that a view references, or
   that a trigger on another table references, needs an `exec`
   dropping and recreating them around it; a trigger on the rebuilt
   table itself goes with it; the fold surfaces both.
10. **`create index`, `drop index`.** The index is identified by its
    table and key exactly as in a schema `indexes` block, so the
    schema and the migration name the same index the same way: the
    DDL mapping assigns the name from the table and the key columns,
    `name:` overrides it, and an index with an expression atom MUST
    carry `name:` (§indexes). `where:` takes an expression literal and
    makes the index partial. `pk` is not a setting of these
    operations: a primary key is a table constraint, declared in a
    `create table` or `rebuild table` body.
11. **`exec`.** The string is one or more SQL statements run
    verbatim, for whatever the operations above do not say: a data
    fix, a view, a trigger, a `PRAGMA`. A foreign key over several
    columns has no shape item; a table that needs one is written
    with `exec`, its `CREATE TABLE` verbatim, and the drift report
    says so. `down:` is its inverse. With `no_transaction` on the
    migration, `exec` statements run outside any transaction, each
    on its own.
12. **Reversibility.** A flat body runs forward as written and
    backward as the inverses of its operations in reverse order. The
    inverses are: `create table` and `drop table` with a body;
    `rename table` and `rename table` reversed; `add` and `drop` with
    a type, whose shape is the inverse `add` and MUST satisfy rule
    8's `add` rules, in a flat body alone, where the inverse is
    derived from it; `rename` and `rename` reversed; `create index`
    and `drop index`; `exec` with `down:` and its `down:`. A flat
    body with an operation that has no inverse (`drop table` without
    a body, `drop` without a type, `rebuild table`, `exec` without
    `down:`) is an error naming that operation, reported at the item
    inside an `alter table`, unless the migration is marked
    `irreversible` or written as `up { } down { }`. A `down` block
    runs as written; an `up` block without a `down` is irreversible.
    Inside the blocks nothing is derived: a typed `drop` there is a
    column, checked as one, not an inverse `add`.
13. **Transactions and foreign keys.** A migration runs in one
    transaction with the ledger write (§migration-ledger). A
    migration that contains a `rebuild table` or a `drop table` is
    bracketed by `PRAGMA foreign_keys = OFF` before `BEGIN`, `PRAGMA
    foreign_key_check` before `COMMIT`, and `PRAGMA foreign_keys =
    ON` after; a non-empty check result rolls the migration back. The
    tool does this; a migration never spells it. `no_transaction`
    disables the bracket and the transaction, for an `exec` that
    needs it (another `PRAGMA`, `VACUUM`).

### The fold

Migrations are untyped so that they may say anything that was once
true. What is checked instead is their composition.

1. **Definition.** The fold of a package is the structure of an empty
   in-memory SQLite database after every migration of the package has
   been applied in name order. The **target** is the structure of an
   empty in-memory database after the schema's generated DDL
   (§mapping-to-sqlite-ddl) has been applied.
2. **Structure** is, per table: its name; its columns as a set, each
   with name, declared type, `NOT NULL`, default as SQLite reports it
   (a default of `NULL`, in any case, read as none, since it means
   what no default means, §default-values) and primary-key membership
   (`PRAGMA table_xinfo`); its foreign keys
   (`PRAGMA foreign_key_list`); its indexes with name, uniqueness,
   partial predicate and key (`PRAGMA index_list`, `PRAGMA
   index_xinfo`). The index of a `UNIQUE` column constraint (SQLite's
   autoindex) is matched by key against any unique index without a
   predicate; every other index is compared by name. Nothing else is
   compared: not column order, not row data, not SQL text, not the
   order of statements, and never the ledger table
   (§migration-ledger).
3. **Drift.** `volt migration check` compares fold and target and
   reports every difference, table by table, as the operations that
   would close it. Zero differences is the success condition. `volt
   gen` runs the same check for a package that declares at least one
   migration and fails on drift, so a schema edit without its
   migration never generates; a package without migrations is
   unmigrated and generates as before.
4. **Lemma.** Because the fold runs on the engine that will run the
   migrations, a migration SQLite would refuse (dropping an indexed
   column, say) is refused at check time, with SQLite's own message.
   The one run-time exception is a NOT NULL column added to a
   table that already has rows, which the fold's empty database
   cannot see and `alter table` (§migrations) guards by requiring a
   default. Once the fold is read, the check also reverts the
   reversible migrations above the last irreversible one, from the
   top down, and applies them again in the fold database, so an
   inverse SQLite would refuse is refused at check time as well; a
   down cannot pass an irreversible migration, so the migrations
   below it are not run backward.
5. **Generation.** The drift report is the input of a migration
   generator (not built): it writes the operations that close the
   drift as a new `Migration` for the user to edit. A rename is a
   drop and an add until the user says otherwise. The generator is a
   tool over the language, not part of it.

### Migration ledger

1. **The table.** Applied migrations are recorded in
   `volt_migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT
   NULL)`, created on first use; `applied_at` is the time of
   application in RFC 3339, UTC. The row is written in the
   migration's transaction, or, with `no_transaction`, after its last
   statement.
2. **Pending** is every migration of the package whose name has no
   row, including one that sorts before an applied one; `status`
   lists such a migration as out of order and `up` applies it anyway,
   in name order among the pending.
3. **Down** removes the applied migration with the greatest name by
   running its inverse (§migrations, reversibility) and deleting its
   row; an irreversible migration refuses with its name.

### Migration tooling

1. **CLI.** `volt migration new LABEL [DIR]` writes
   `<name>.volt` holding the package clause and an empty `Migration
   <name> { }` into the package at `DIR` (the current directory when
   absent); the user may move the element to any file of the
   package. `volt migration check [DIR]` is the fold (§the-fold).
   `volt migration up|down|status DSN [DIR]` operate on the SQLite
   database at `DSN`, a file path or a `file:` URI: `up` creates the
   database when it is missing, `status` and `down` open an existing
   one only (a missing file is an error and nothing is created), and
   all three refuse `:memory:`. `volt migration sql [DIR]` prints
   every migration's statements in both directions for review. Exit
   codes, the same from `gen`: 1 for drift and for an error in the
   migrations themselves, a statement SQLite refuses in either
   direction, an irreversible migration asked to go down, a migration
   that does not lower; 2 for usage and I/O, a database that cannot
   be opened, is locked past its busy timeout or fails otherwise. The
   tool carries its own SQLite (the `mattn/go-sqlite3` driver) and
   needs cgo, a C compiler at build time, for it; a `volt` built
   without cgo refuses the migration commands and the fold with a
   message saying so. The library and the runtime carry no SQLite.
2. **Generated.** For a package that declares migrations, `volt gen`
   writes `volt_migrations.go` beside the other generated files:
   `var Migrations = []rt.Migration{…}` in name order, each with its
   name, up statements, down statements (nil when irreversible) and
   its transaction and foreign-key bracket flags; and `func
   Migrate(ctx context.Context, db *sql.DB) error`, which applies
   every pending migration through `rt.MigrateUp` and is called by
   nothing unless the application calls it. The file belongs to the
   queries part (`-parts queries`) and is absent with `-parts
   models`. With it, a binary migrates its own database and a test
   builds a fresh one with one call, without the `volt` tool or the
   `.volt` files.
3. **Runtime.** `rt.Migration` is the statement form of one
   migration; `rt.MigrateUp(ctx, db, migrations)` applies the pending
   ones and returns their names; `rt.MigrateDown(ctx, db, migrations)`
   reverts the last applied one and returns its name; `rt.
   MigrateStatus(ctx, db, migrations)` lists each migration with
   whether and when it was applied and whether it is out of order,
   and every ledger row that no migration of the package names,
   marked as such. The CLI's `up`, `down` and `status` are these
   three over the statements lowered from the `.volt` files. A
   migration that does not run, a statement SQLite refuses, a failed
   foreign-key check or a down asked of an irreversible migration, is
   an `*rt.MigrationError` naming the migration, the direction and
   the statement, so that a caller tells it from the database
   failing.
4. **No implicit schema change.** Neither `volt gen` nor the runtime
   ever alters a database from the schema alone. The schema is the
   target; a migration is the only path to it.

```volt fragment
package db

Table users {
  id        int    [pk, increment]
  email     string [unique, required]
  full_name string [required]
  bio       string [null]
  hits      int    [default: 0]

  validators {
    EmailShape(email)
    HitsSane(hits)
  }
}

Migration 0001__create_users {
  create table users {
    id    int    [pk, increment]
    email string [unique]
    name  string
    hits  string [default: '0']
  }
}

Migration 0002__users_bio_and_int_hits [irreversible] {
  alter table users {
    add bio string [null]
    rename name to full_name
  }
  rebuild table users {
    id        int    [pk, increment]
    email     string [unique]
    full_name string
    bio       string [null]
    hits      int    [default: 0, from: `CAST(hits AS INTEGER)`]
  }
}
```

The `validators` block and the two `required` settings changed
nothing in the database and appear in no migration; they changed
`UserChangeset.Validate`. The second migration is irreversible
because the rebuild forgets the old type of `hits`; the first is
reversible on its own.

---

## Collected grammar (Part II)

Additions to Collected grammar (Part I). The
`element` production is extended:

```ebnf
element        = (* Collected grammar (Part I) alternatives *)
               | package clause | import decl | pipeline | scope
               | group decl | pred decl | select decl | migration ;

slash          = "/" ;

package clause = "package", name, newline ;

import decl    = "import", "(", import entry, { import entry }, ")" ;
import entry   = [ alias ], package path, newline ;
alias          = name ;
package path   = name, { slash, name } ;

pipeline       = "Pipeline", name, "{", { plug }, "}" ;
plug           = "use", go ref, newline ;
go ref         = name, [ ".", name ] ;

scope          = "Scope", route path, [ settings ],
                 "{", { scope item }, "}" ;
scope item     = route | resources | dataset | scope ;

route          = verb, route path, handler ref, [ settings ], newline ;
verb           = "get" | "post" | "put" | "patch" | "delete"
               | "options" | "head" | "any" ;
handler ref    = name, [ ".", name ] ;

route path     = slash, [ segment, { slash, segment } ] ;
segment        = name
               | ":", name, [ "(", param type, ")" ]
               | ":", name, "." , ".", "." ;
param type     = "int" | "string" ;

resources      = "resources", table ref, [ settings ], newline ;
table ref      = name, [ ".", name ] ;
dataset        = "dataset", [ name, "." ], name, [ settings ], newline ;

migration          = "Migration", migration name, [ migration settings ],
                     "{", migration body, "}" ;
migration name     = digit, digit, digit, digit, { digit }, "__", label ;
label              = ( letter | digit ), { letter | digit } ;
migration settings = "[", migration setting, { ",", migration setting }, "]" ;
migration setting  = "irreversible"
                   | "no_transaction" ;
migration body     = { operation }
                   | "up", "{", { operation }, "}",
                     [ "down", "{", { operation }, "}" ] ;
operation          = create table op | drop table op | rename table op
                   | alter table op  | rebuild table op
                   | create index op | drop index op
                   | exec op ;
create table op    = "create", "table", name, "{", shape body, "}" ;
drop table op      = "drop", "table", name, [ "{", shape body, "}" ], newline ;
rename table op    = "rename", "table", name, "to", name, newline ;
alter table op     = "alter", "table", name, "{", { alter item }, "}" ;
alter item         = "add", migration column
                   | "drop", name,
                     [ column type, [ migration column settings ] ], newline
                   | "rename", name, "to", name, newline ;
rebuild table op   = "rebuild", "table", name, "{", shape body, "}" ;
create index op    = "create", "index", name, index key,
                     [ index settings ], newline ;
drop index op      = "drop", "index", name, index key,
                     [ index settings ], newline ;
exec op            = "exec", string, [ exec settings ], newline ;
exec settings      = "[", "down", ":", string, "]" ;
shape body         = { migration column | indexes block } ;
migration column          = name, column type,
                            [ migration column settings ], newline ;
migration column settings = "[", migration column setting,
                            { ",", migration column setting }, "]" ;
migration column setting  = column setting
                          | "from", ":", expression literal
                          | "delete", ":", ref action
                          | "update", ":", ref action ;

(* setting value, extended (Part I §settings-lists): *)
setting value  = (* schema-layer alternatives *) | ident list | order list ;
ident list     = "(", name, { ",", name }, ")" ;
```

The `group decl`, `pred decl` and `select decl` productions — with the
predicate expression grammar they share — are collected in §groups, §predicates
and §selects-over-groups, and the validators block in §validators; they are not
duplicated here. The `order list` that extends `setting value` above is the
value of a Select's `order:` and is defined in §selects-over-groups.

All tokens of a `route path` and of a `package path` MUST be contiguous
(§route-paths, §imports) — the grammar above is subject to that adjacency
constraint, which the token stream expresses via inter-token whitespace
flags.

---

## Conformance and the proof chain

Testing never proves software correct; a specification with an
executable conformance surface narrows the gap deliberately. The
chain, each link runnable by `go test ./...`:

1. **Corpus ↔ spec.** Every snippet in `lang/conformance/snippets/`
   — a `.volt` file, or a directory whose first `.volt` file
   carries the tag — cites a `// spec: §…` section; `valid/` MUST check
   clean, `invalid/` MUST be rejected, and an invalid snippet with a
   `// want: <text>` line MUST be rejected with a diagnostic containing
   that text — for the rule it names, not by accident. Content, not
   name, tells the layers apart: a snippet file without a `package`
   clause is a Part I file body and runs through the schema pass alone
   (`lang/check`); a file with a package clause, and every directory,
   runs through the project pass. The corpus is the spec's executable
   surface.
2. **Schema layer preserved.** The Part I corpus runs against the same
   front end as the full language, so the layering rule
   (§layers-files-and-the-superset-rule) is enforced, not assumed.
3. **Generator ↔ contract.** Goldens are byte-compared, gofmt-stable by
   test, and **compiled by the real Go toolchain** against stub
   implementations of the generated interfaces — the typed-signature
   contract (§handler-references) is checked by the Go compiler itself.
4. **Router semantics.** The `itest/` fixture app is generated,
   committed, drift-checked against regeneration, and exercised over
   httptest: match and 404/405 behavior, typed-parameter 404s (§route-paths),
   pipeline order (§scopes), the error spine (§scopes), and
   **reverse-URL round-trip totality** — for every route in the
   generated table, the URL built by its helper is served back to the
   router and MUST dispatch to that same route. The §route-names-and-reverse-urls property is
   not sampled; it is enumerated.
5. **Grammar ↔ front end.** The EBNF of this document is read by test
   (`lang/ebnf`): every block parses, every name is defined once, the
   collected grammars repeat the sections exactly, and from the
   grammar and the lexical rules stated beside it (§tokenization, §line-structure-and-whitespace, §settings-lists,
   §route-paths) a sentence is derived for every choice in every
   production and MUST parse; every text one token away from one of
   them (a token dropped, inserted, or two exchanged) MUST be accepted
   by the front end exactly when the grammar accepts it. The grammar
   is not a picture of the parser; it is checked against it.
6. **Examples ↔ front end.** Every `volt` block of this document runs
   (`lang/spec_examples_test.go`): a plain block is a whole file the
   front end accepts, through the schema pass without a package clause
   and through the project pass with one, and a Part II declaration in
   a plain block needs its package clause; a block whose fence reads
   `volt invalid` MUST be rejected; a block whose fence reads
   `volt fragment` is a piece of a file, shown without its package
   clause or the Go functions and packages it names, and MUST parse
   alone. An example cannot drift from the language it shows.

---

# Appendices

## Mapping to Go

*The data layer: generation contract of `volt gen` for schema packages (`nao_models.go`,
`nao_queries.go`, `nao_dyn.go`). Informative in form; in force it is
pinned by the generator goldens, which are gofmt-stable and compiled by
the real Go toolchain on every `go test ./...` run.*

`nao_models.go` carries every type the wire carries that Volt
generates: the row structs, the changesets with their column handles
and `<Model>Params` (§changesets, generation contract), so a program
that shares the models, a generated client (§generated-client) among
them, needs nothing else but the declared types the models name: a
package-qualified one from its own package, a bare one from the data
package's Go files (§types).
`nao_queries.go` carries the `Queries` handle and its methods, and
imports only what their signatures name.

### Go types

A column type is a Go type (§columns), so the mapping is the identity
for the built-ins; the checker has already refused anything outside
the set.

| Declared | Go |
|---|---|
| `bool`, `int`, `float64`, `string`, `time.Time` | the same |
| `[]byte` | `[]byte`; nil is NULL, never wrapped |
| declared `Type pkg.Name` (§types) | `pkg.Name`, imported as `import pkg "path"` |
| declared `Type Name` (§types) | `Name`, the data package's own type, named unqualified |

`int` is Go's `int`: Volt targets 64-bit platforms (amd64 and arm64),
so a route parameter or primary key of type `int` carries the full
SQLite INTEGER range there, and only there.

### Nullability

A column that carries `null` (§columns) is nullable and generated as
`rt.Null[T]`, a value plus a validity bit, never a pointer. Every other
column is NOT NULL and generated as `T`.
`[]byte`, where `nil` already expresses NULL, stays bare either way, so
a `[]byte` column without `null` refuses nil, its Go zero value, as a
`not_null` field error (§changesets), and a non-nil empty slice stores
empty bytes (§columns). GOB does not carry an empty slice, so an
empty slice in a GOB write body (§formats) arrives as nil, which is
NULL: stored as NULL in a column that carries `null`, refused in one
without. A nullable
column's handle, declared in `nao_models.go` with the rest
(§changesets, §generated-query-surface), is an `rt.NullColumn`:
comparisons take plain `T`, NULL is explicit (`IsNull`, `SetNull`).

### Names

`snake_case` → `PascalCase` with the Go initialisms convention
(`user_id` → `UserID`, `api_key` → `APIKey`). Model names are the
singular of the table name (`users` → `User`), overridable with
`[model:]` — the only model-naming override; `resources`' `singular:`
setting (§settings) renames the route member helper, never the model.
A leading digit is
prefixed (`2fa_codes` → `X2faCode`). Two declarations mapping to the
same Go identifier is a generation error; the `dynname` lint reports
the dynamic layer's concatenation collisions ahead of time. A bare
declared type shares this scope, since the data package's own Go
files declare it. Its name is exported, so the unexported names of
generated code never meet it, and one that takes an exported
generated name, a router's included, is a check error (§types).

### Notes become doc comments

Every note is the `note:` setting (§notes):

| Note on | Generated position |
|---|---|
| `Project` | package comment |
| `Table` | struct doc comment |
| column | field doc comment |
| injected partial column | travels with the column; a §tablepartial override brings its own note |

Generated files start with the machine-readable
`// Code generated … DO NOT EDIT.` marker; `volt gen` refuses to
overwrite a file lacking it. Generated code imports the standard
library, `nao/rt` and the packages of declared types (§types) only.

### Struct tags

Every generated struct field derived from a column carries the same
tag, assembled in this order:

1. `db:"<column name>"` — the scan contract. Not overridable; the
   column setting `tag:` rejects the key `db` (§columns).
2. `json:"<column name>"` — the JSON default. Nothing is omitted
   (`omitempty` is never generated): a document always carries every
   column, and NULL is the value `null`. A column `tag:` with key
   `json` **replaces** this pair verbatim.
3. Every other `tag:` passthrough pair, verbatim, in declaration order.

A struct derivative (§selects-over-groups) copies each kept field's assembled tag
unchanged; an explicit-list shared row type (§selects-over-groups) carries the
default `db`/`json` pair only.

### Generated query surface

Per table, on the `Queries` handle of `nao_queries.go` (`New(db
rt.DBTX)`, `Tx`, `WithTx`, `WithCache`):

1. `<Model>Get`, `<Model>Update` and `<Model>Delete` exist iff the
   table has a primary key (a `pk` column or a `[pk]` index), one
   positional argument per key column; `Update` also needs a non-key
   column; `Create` and `Update` take a changeset and return the
   stored row (§changesets). `<Model>GetMany(ctx,
   keys)` exists for a single-column key; missing keys are absent, not
   errors; an empty input returns nil. `<Model>GetBy<Column>` exists
   per column that is `unique` or covered by a single-column unique
   index without `where:` and is not the key (a partial unique index
   constrains only some rows, §indexes). `<Model>List` and
   `<Model>Create` always exist; a table without a key gets List,
   Create, GetBy and the dynamic layer.
2. `Get`, `GetBy*` and `Update` return `rt.ErrNotFound`
   (`sql.ErrNoRows`) on a miss; `Delete` checks the affected count.
   Multi-row results are empty slices, never nil, GetMany on an empty
   input excepted.
3. Every statement is rendered by the runtime (§predicates, rendering):
   `Get`, `GetBy*`, `List`, `Delete` and the select methods are
   constants rendered at generation time, bound positionally in the
   order the text names their parameters; `GetMany` renders at run
   time, its `IN` list varying with the input, and does not use the
   statement cache. `Create` and `Update` are rendered at run time
   from the changeset's marks and end in `RETURNING` (§changesets).
   Every CRUD write's database refusal becomes `*rt.Invalid`;
   `DeleteWhere` and `UpdateWhere` return the driver's error.
4. The column handles `<Model><Field>` (`rt.Column[M, T]`,
   `rt.NullColumn` when nullable) are declared in `nao_models.go`
   (§changesets); the dynamic layer in `nao_dyn.go` mints per model the
   wrappers `Limit`, `Offset`, `Distinct`, `OrderBy`,
   `After`, `Set`, and the verbs `Query`, `Count`, `Exists`,
   `DeleteWhere`, `UpdateWhere`. Predicates are values rendered by the
   one renderer (§predicates, rendering): an empty `In` is constant false,
   a zero predicate vanishes, a `DeleteWhere` or `UpdateWhere` with no
   predicate is an error, `After` is keyset pagination over the
   `OrderBy` terms. Dynamic statements run through `rt.StmtCache` when
   `WithCache` was set, the changeset writes among them; the static
   CRUD and select methods do not use the cache;
   `WithTx` drops it.
5. `rt.Open(driver, dsn)` pins one connection and sets WAL,
   `busy_timeout = 5000` and `foreign_keys = ON`. `rt.Null[T]`
   marshals as the value or `null`.

---

## Mapping to SQLite DDL

*Generation contract of `volt gen --sql` (`nao_schema.sql`). Pinned by
goldens that are executed on a real SQLite (`PRAGMA foreign_keys = ON`,
`foreign_key_check` after), seed INSERTs included.*

| Declared | SQLite |
|---|---|
| column without `null`, key columns included (SQLite's `PRIMARY KEY` does not imply NOT NULL) | `NOT NULL`, the rowid alias below excepted; a column that carries `null` has no nullability constraint (§columns) |
| `int` `[pk, increment]` | exactly `INTEGER PRIMARY KEY` (rowid alias, auto-assigned); SQLite's `AUTOINCREMENT` keyword deliberately avoided — it only adds the never-reuse-an-id bookkeeping the SQLite docs discourage. Any other `increment` placement is a check error (§columns) |
| composite pk (`[pk]` index) | `PRIMARY KEY (a, b)` table constraint; members get `NOT NULL` |
| index | `CREATE INDEX <name> ON <table> (<key>)`, `UNIQUE` for `unique`, `WHERE <expr>` for `where:` (§indexes); the name is `name:` when given, else `idx_<table>_<col>[_<col>...]` from the SQLite table name and the key columns; two indexes of one table that derive one name are a generation error |
| relationship (`Ref:`, `ref:`) | `FOREIGN KEY (<fk>) REFERENCES <table> (<columns>)` with `ON DELETE` / `ON UPDATE`; the foreign key is the left endpoint of a `Ref` and the declaring column of a `ref:` (§relationships-ref) |
| one-to-one (a unique foreign key, §relationships-ref) | the uniqueness as written, `UNIQUE` on the column or the unique index; the relationship adds nothing |
| `int`, `bool` | `INTEGER` (booleans stored 0/1; `true`/`false` in records and defaults become 1/0) |
| `float64` | `REAL` |
| `string` | `TEXT` |
| `time.Time` | `TIMESTAMP` (ISO-8601 text; the DDL spelling is what the sqlite3 driver keys its time conversion on) |
| `[]byte` | `BLOB` |
| declared `Type` (§types) | the column type of its storage type; a value set (§types) carries no `CHECK`, its values are the Go constants |
| backtick expressions | verbatim; parenthesized in `DEFAULT` as SQLite requires |
| validators, `required` | nothing; the database never carries a `CHECK` (§validators) |
| `records` | multi-row `INSERT` statements after all tables, in source order; `null` becomes `NULL` |
| unknown type | check error naming the column (§columns); the generator never sees one |

Identifiers are quoted only when needed (not a plain lower-case name,
or a SQLite keyword): `order` → `"order"`, `full name` → `"full name"`,
`2fa_codes` → `"2fa_codes"`. Project note → file header comment; table
and column notes → comments above their DDL. Output opens with
`PRAGMA foreign_keys = ON;` and the `-- Code generated … DO NOT EDIT.`
marker, with the same overwrite protection as §mapping-to-go.

---

## Non-goals

Volt does not generate and will not generate: an identity map or unit
of work, lazy loading, active-record `Save()`, callbacks, dirty
tracking, default scopes, single-table inheritance, reflection in
generated code, a Turing-complete query language. Generated code
imports the standard library, Volt's runtime packages and the packages of declared types only.

## License and Provenance

Part I is a distillation of the DBML language as documented and
implemented in [holistics/dbml](https://github.com/holistics/dbml)
(the `@dbml/parse` reference implementation). While Part I was being
written, every corpus verdict was cross-checked against that
implementation and the prose corrected wherever it disagreed; the
cross-check harness was retired at zero disagreements, and the
corpus it pinned remains the executable record.
Like the upstream project, this specification is licensed under the
[Apache License 2.0](../LICENSE).
