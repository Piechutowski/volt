# The Volt Language — Specification

**Status:** Normative. The implementation in this repository MUST be
100% compliant with this document; where they disagree, one of the two
has a bug, and the commit that fixes it says which.

Volt is **one language**: a declarative complement to Go, grown outward
from DBML. This specification has two parts and three appendices:

- **[Part I — The Schema Core (DBML)](#part-i--the-schema-core-dbml)**
  (§1–§8): lexis, grammar and static semantics of the DBML-derived
  schema layer. Standalone-conformant with upstream DBML.
- **[Part II — The Project and Routing Layer (§V)](#part-ii--the-project-and-routing-layer-v)**
  (§V0–§V8): projects, packages, imports, pipelines, scopes, routes,
  resources.
- **Appendix A — Mapping to Go** and **Appendix B — Mapping to SQLite
  DDL**: the generation contracts of the data layer (informative in
  form, pinned by generator goldens in force).
- **Appendix C — Compatibility with DBML**: what Volt accepts verbatim,
  the one deliberate break (`use`/`reuse`), and the migration.

Every construct is specified by (1) a grammar production in EBNF, (2)
an enumerated list of constraints, and (3) a minimal example. Every
constraint is executable: the conformance corpus under
[`lang/conformance/snippets/`](../lang/conformance/snippets/) tags each snippet with the
section it exercises (`// spec: §…`); `valid/` MUST be accepted,
`invalid/` MUST be rejected, and `go test ./...` runs the whole chain.
Diagnostics cite the section they enforce. The lint annex — legal but
suspicious schemas, every rule with executable examples — is
[`lint.md`](lint.md); lint rules never change what conforming Volt is.

---

# Part I — The Schema Core (DBML)

**Version:** 1.0 (based on `@dbml/parse` reference implementation, holistics/dbml)
**Status:** Normative.
This part is a complete, standalone-conformant specification of DBML:
a schema written using only Part I constructs is valid upstream DBML
(diagrammable on dbdiagram.io as-is). The reference implementation is
[`lang/`](../lang/); the executable corpus is
[`lang/conformance/snippets/`](../lang/conformance/snippets/) (the `.dbml` entries).

DBML (Database Markup Language) is a declarative, database-agnostic domain-specific
language for defining database schemas: tables, columns, indexes, constraints,
relationships, enumerations, sample data, and documentation metadata.

This document is a complete, formal specification of DBML intended for
implementers of parsers, compilers, and tooling. Every construct is specified
by (1) a grammar production in EBNF, (2) an enumerated list of constraints, and
(3) a minimal example. The collected grammar appears in [Appendix IA](#appendix-ia-collected-grammar-part-i).

---

## Table of Contents

1. [Notation](#1-notation)
   - 1.1 [EBNF](#11-ebnf)
   - 1.2 [Character Notation (U+XXXX)](#12-character-notation-uxxxx)
   - 1.3 [Unicode General Categories](#13-unicode-general-categories)
   - 1.4 [Case Sensitivity and Terminology](#14-case-sensitivity-and-terminology)
2. [Source Text](#2-source-text)
3. [Lexical Structure](#3-lexical-structure)
   - 3.1 [Tokenization](#31-tokenization)
   - 3.2 [Line Structure and Whitespace](#32-line-structure-and-whitespace)
   - 3.3 [Comments](#33-comments)
   - 3.4 [Identifiers](#34-identifiers)
   - 3.5 [Keywords](#35-keywords)
   - 3.6 [String Literals](#36-string-literals)
   - 3.7 [Multi-line String Literals](#37-multi-line-string-literals)
   - 3.8 [Escape Sequences](#38-escape-sequences)
   - 3.9 [Numeric Literals](#39-numeric-literals)
   - 3.10 [Boolean and Null Literals](#310-boolean-and-null-literals)
   - 3.11 [Color Literals](#311-color-literals)
   - 3.12 [Expression Literals](#312-expression-literals)
   - 3.13 [Operators and Punctuation](#313-operators-and-punctuation)
4. [Common Syntactic Forms](#4-common-syntactic-forms)
   - 4.1 [Names and Qualified Names](#41-names-and-qualified-names)
   - 4.2 [Settings Lists](#42-settings-lists)
5. [Program Structure](#5-program-structure)
6. [Element Definitions](#6-element-definitions)
   - 6.1 [Project](#61-project)
   - 6.2 [Table](#62-table)
   - 6.3 [Columns](#63-columns)
   - 6.4 [Default Values](#64-default-values)
   - 6.5 [Indexes](#65-indexes)
   - 6.6 [Checks](#66-checks)
   - 6.7 [Relationships (Ref)](#67-relationships-ref)
   - 6.8 [Enum](#68-enum)
   - 6.9 [TablePartial](#69-tablepartial)
   - 6.10 [Records (Sample Data)](#610-records-sample-data)
   - 6.11 [Notes](#611-notes)
   - 6.12 [TableGroup](#612-tablegroup)
   - 6.13 [DiagramView](#613-diagramview)
7. [File Imports (removed)](#7-file-imports-removed)
8. [Static Semantics](#8-static-semantics)
- [Appendix IA: Collected Grammar](#appendix-ia-collected-grammar-part-i)

---

## 1. Notation

### 1.1 EBNF

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
meta-language* are distinct from the DBML tokens `[ ]`, `{ }`, `( )`, which
always appear quoted (`"["`, `"{"`, `"("`) when they are part of the language
being defined.

Whitespace between symbols in a production is insignificant unless a
production explicitly references the `newline` or `sp` nonterminals.

### 1.2 Character Notation (U+XXXX)

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

### 1.3 Unicode General Categories

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

### 1.4 Case Sensitivity and Terminology

**Case sensitivity.** All DBML *keywords* (terminals spelled with letters in
this grammar, e.g. `"Table"`, `"pk"`, `"not null"`) are matched
**case-insensitively**: `Table`, `table`, and `TABLE` are equivalent.
User-defined names (identifiers) preserve case; implementations MUST treat
them case-sensitively for lookup unless stated otherwise.

**Terminology.** The key words MUST, MUST NOT, SHOULD, and MAY are to be
interpreted as described in RFC 2119.

---

## 2. Source Text

1. A DBML source file is a sequence of Unicode characters, conventionally
   encoded as UTF-8, conventionally using the file extension `.dbml`.
2. A *program* (§5) is the content of one source file. Multiple files are
   related only through the module system (§7).

---

## 3. Lexical Structure

### 3.1 Tokenization

Before parsing, the input is split into *tokens* (identifiers, literals,
operators, punctuation) by a scanner governed by two rules:

1. **Left to right, single pass.** The scanner starts at the first character
   and repeatedly cuts the next token off the front of the remaining input.
   It never backs up to re-tokenize text it has already consumed.
2. **Longest match** (also called *greedy* or *maximal munch*). If, at the
   current position, the upcoming characters could form more than one valid
   token, the scanner always chooses the **longest** one.

Consequences of the longest-match rule:

- `<>` is a single many-to-many operator, never the two tokens `<` `>`.
- `'''` opens a multi-line string, never the empty string `''` followed by `'`.
- `//` begins a comment, never two `/` operators; `>=` is one token, not `>` `=`.
- `user_id2` is one identifier, not the identifier `user_id` followed by the
  number `2` — the scanner keeps consuming while characters can extend the
  current token.

To write two adjacent tokens that would otherwise fuse, separate them with
whitespace.

### 3.2 Line Structure and Whitespace

```ebnf
newline = ? U+000A LINE FEED ? ;
sp      = ? U+0020 SPACE ? | ? U+0009 TAB ? ;
```

1. Carriage return (U+000D) is discarded wherever it appears and produces no
   token; files with Windows (CRLF) line endings are therefore handled
   transparently.
2. DBML is **newline-sensitive**: a line break terminates statements such as
   column definitions, enum values, record rows, and settings-free field
   lines. Productions in this specification reference `newline` explicitly
   wherever it is syntactically significant.
3. Space and tab characters separate tokens and are otherwise insignificant.
4. Indentation is never significant (except inside multi-line strings, §3.7).

### 3.3 Comments

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

### 3.4 Identifiers

```ebnf
identifier        = plain identifier | quoted identifier ;

letter            = ? any character of Unicode category L (Letter) ?
                  | ? any character of Unicode category M (Mark) ?
                  | "_" ;
digit             = "0" | "1" | "2" | "3" | "4"
                  | "5" | "6" | "7" | "8" | "9" ;
ident char        = letter | digit ;

plain identifier  = letter, { ident char }
                  | digit, { ident char } ;    (* see constraint 2 *)

quoted identifier = '"', { qi char | escape sequence }, '"' ;
qi char           = any char - ( '"' | "\" | newline ) ;
```

1. A plain identifier is a maximal run of letters, digits, combining marks,
   and underscores.
2. A plain identifier MAY begin with digits, but a token consisting only of
   digits (and at most one `.`) is a numeric literal (§3.9), never an
   identifier. E.g. `2fa_codes` is an identifier; `255` is a number.
3. A quoted identifier (`"double quoted"`) permits any characters except an
   unescaped `"` and a line break, and supports the escape sequences of §3.8.
   Use it for names containing spaces or other special characters, including
   column types with spaces (e.g. `"double precision"`).
4. Plain and quoted identifiers are interchangeable everywhere an
   `identifier` is expected; `users` and `"users"` denote the same name.

### 3.5 Keywords

DBML has **no reserved words**. All keywords are contextual: a keyword such
as `Table` acts as a keyword only in keyword position and remains usable as an
ordinary identifier elsewhere (e.g. a column may be named `table`).

Element-type keywords: `Project`, `Table`, `TablePartial`, `TableGroup`,
`Enum`, `Ref`, `Note`, `Records`, `DiagramView`, `indexes`, `checks`,
`Tables`, `Notes`, `TableGroups`, `Schemas`.
Clause keywords: `as`, `use`, `reuse`, `from`.
Value keywords: `true`, `false`, `null`.
All are case-insensitive (§1.4).

### 3.6 String Literals

```ebnf
string             = single line string | multi line string ;

single line string = "'", { sls char | escape sequence }, "'" ;
sls char           = any char - ( "'" | "\" | newline ) ;
```

1. A single-line string is delimited by single quotes `'` and MUST NOT
   contain an unescaped line break.
2. Escape sequences (§3.8) are interpreted.

### 3.7 Multi-line String Literals

```ebnf
multi line string = "'''", mls body, "'''" ;
mls body          = { ( any char - "\" ) | escape sequence }
                    - ( { any char }, "'''", { any char } ) ;
```

1. Delimited by triple single quotes `'''`; may span any number of lines.
2. Escape sequences (§3.8) are interpreted; escape a literal `'` as `\'` and
   a literal `\` as `\\`.
3. A backslash immediately before a line break is a **line continuation**:
   both the backslash and the line break are removed from the value.
4. **Indentation stripping:** after escape processing, compute the minimum
   number of leading spaces over all non-empty lines; remove exactly that
   many leading spaces from every line. A first line that is empty (the
   common case, where content starts on the line after the opening `'''`)
   and a trailing newline before the closing `'''` are removed.

```volt
Note: '''
  This is a block string.
  It spans multiple lines.
'''
```

The value of the above is `This is a block string.\nIt spans multiple lines.`

### 3.8 Escape Sequences

Escape sequences apply inside single-line strings, multi-line strings, and
quoted identifiers. They do **not** apply inside expression literals (§3.12)
or comments.

```ebnf
escape sequence = "\", escaped item ;
escaped item    = "t" | "n" | "r" | "0" | "b" | "v" | "f"
                | "\" | "'" | '"' | "`"
                | newline
                | "u", 4 * hex digit
                | any char ;
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

### 3.9 Numeric Literals

```ebnf
number   = digit, { digit }, [ ".", digit, { digit } ], [ exponent ] ;
exponent = ( "e" | "E" ), [ "+" | "-" ], digit, { digit } ;
```

1. Examples: `42`, `3.14`, `1e2`, `1.5e10`, `3.14e-5`.
2. A leading sign is not part of the literal; a negative value such as
   `-100` in record rows is parsed as prefix operator `-` applied to a number.
3. At most one decimal point is permitted. A digit run followed by further
   letters (e.g. `2fa`) lexes as an identifier (§3.4); a digit run containing
   a dot followed by letters (e.g. `12.3abc`) is a lexical error.

### 3.10 Boolean and Null Literals

```ebnf
boolean = "true" | "false" ;
null    = "null" ;
```

Case-insensitive, as all keywords.

### 3.11 Color Literals

```ebnf
color = "#", ( 3 * hex digit | 6 * hex digit ) ;
```

1. Shorthand `#rgb` or full `#rrggbb` hexadecimal color, e.g. `#3498DB`.
2. Used as the value of `headercolor` and `color` settings.

### 3.12 Expression Literals

```ebnf
expression literal = "`", { any char - "`" }, "`" ;
```

1. Delimited by backticks. The content is an opaque, raw SQL expression:
   backslash is **not** an escape character and there is no way to embed a
   literal backtick.
2. May span multiple lines.
3. Used for computed defaults (`` default: `now()` ``), expression indexes,
   check expressions, and expression values in records.

### 3.13 Operators and Punctuation

```ebnf
rel op = "<>" | "<" | ">" | "-" ;
punct  = "{" | "}" | "[" | "]" | "(" | ")"
       | "," | ":" | ";" | "." | "~" | "*" ;
```

Punctuation roles (uniform across the language):

| Token   | Role                                                        |
|---------|-------------------------------------------------------------|
| `{ }`   | element bodies (block form)                                 |
| `[ ]`   | settings lists                                              |
| `( )`   | composite column lists, type arguments, records column list |
| `:`     | introduces an inline (single-expression) body or a setting value |
| `,`     | separator inside `[ ]`, `( )`, and record rows              |
| `.`     | name qualification (`schema.table.column`)                  |
| `~`     | TablePartial injection prefix                               |
| `*`     | wildcard (module system, DiagramView)                       |
| `< > - <>` | relationship cardinality operators                       |

---

## 4. Common Syntactic Forms

### 4.1 Names and Qualified Names

```ebnf
name          = identifier ;

schema name   = name ;
table name    = [ schema name, "." ], name ;
column path   = [ schema name, "." ], name, ".", name ;
enum constant = name, ".", name ;        (* EnumName.value *)
```

1. `table name` optionally qualifies a table (or enum) with a schema.
2. If the schema qualifier is omitted, the name belongs to the default
   schema `public` (§8.1).
3. `enum constant` references one value of an enum, e.g. `status.active`;
   it is valid as a default value (§6.4) and as a record value (§6.10).

### 4.2 Settings Lists

Settings attach metadata to the construct they follow. A settings list is
always delimited by square brackets and comma-separated:

```ebnf
settings      = "[", setting, { ",", setting }, "]" ;
setting       = setting name, [ ":", setting value ] ;
setting name  = identifier, { sp, { sp }, identifier } ;
setting value = string | number | boolean | null
              | color | expression literal | identifier
              | inline ref value ;
```

1. A setting is either a flag (`pk`, `unique`, `not null`) or a key–value
   pair (`note: 'text'`, `default: 123`).
2. Setting names are case-insensitive and may consist of multiple words
   separated by spaces (`not null`, `primary key`, `no action`).
3. Each element section below enumerates *which* settings are valid there;
   a setting not listed for a construct is invalid in that position.
4. Within one settings list, each setting MUST appear at most once. The
   only exceptions are the column settings `check` and `ref` (§6.3), which
   MAY be repeated. Settings that are synonyms (`pk` / `primary key`) count
   as one setting for this rule.
5. A settings list MUST appear on the same line as (the end of) the
   construct it modifies.

---

## 5. Program Structure

```ebnf
program = { import statement | element } ;

element = project
        | table
        | table partial
        | enum
        | ref element
        | sticky note
        | table group
        | records element
        | diagram view ;
```

1. A program is a sequence of top-level elements and import statements
   (§7), in any order, separated by line breaks.
2. Forward references are permitted: an element may reference another
   element defined later in the file (or imported). DBML is fully
   declarative; declaration order carries no semantics.

---

## 6. Element Definitions

### 6.1 Project

Declares project-level metadata. There MUST be at most one `Project` element
per compiled schema.

```ebnf
project          = "Project", [ name ], "{", project body, "}" ;
project body     = { project property | note def } ;
project property = identifier, ":", string, newline ;
```

1. `name` names the project; it MAY be omitted.
2. Each property is a free-form key with a string value, one per line.
   The conventional well-known key is `database_type` (e.g. `'PostgreSQL'`,
   `'MySQL'`). Implementations MUST accept arbitrary property keys.
3. A `note def` (§6.11) documents the project.

```volt
Project ecommerce {
  database_type: 'PostgreSQL'
  Note: 'E-commerce database schema'
}
```

### 6.2 Table

```ebnf
table          = "Table", table name, [ table alias ], [ table settings ],
                 "{", table body, "}" ;
table alias    = "as", name ;

table body     = { column
                 | indexes block
                 | checks block
                 | note def
                 | partial injection
                 | records block
                 } ;

table settings = "[", table setting, { ",", table setting }, "]" ;
table setting  = "headercolor", ":", color
               | "note", ":", string ;
```

1. A table MUST contain at least one column, either directly or via an
   injected TablePartial (§6.9).
2. **Alias.** `as name` declares an alternative name usable anywhere the
   table name is (e.g. in `Ref`s). The alias shares the namespace of
   top-level table names and MUST be unique. The alias is not
   schema-qualified.
3. **Settings.** `headercolor` (visualization) and `note` are the only table
   settings.
4. Column names MUST be unique within a table after partial injection (§8.4).

> **Extension (this implementation).** The table setting `model: string`
> pins the Go model name used by `volt gen` (by default the
> singularized table name — [decision D10](decisions.md)). The
> setting is accepted by this front end but is not upstream DBML; the
> `vet` rule [`modelname`](lint.md#modelname) tells you when it is
> needed. The core grammar above is unchanged.

```volt
Table core.users as U [headercolor: #3498DB] {
  id integer [pk]
  email varchar(255) [not null, unique]
}
```

### 6.3 Columns

```ebnf
column          = name, column type, { legacy flag },
                  [ column settings ], newline ;

column type     = type name, [ "(", type arg, { ",", type arg }, ")" ] ;
type name       = [ schema name, "." ], identifier ;
type arg        = number | identifier ;

legacy flag     = "pk" | "unique" ;

column settings = "[", column setting, { ",", column setting }, "]" ;
column setting  = "primary key" | "pk"
                | "null" | "not null"
                | "unique"
                | "increment"
                | "default", ":", default value
                | "check", ":", expression literal
                | "note", ":", string
                | "ref", ":", inline ref value ;
```

1. **Type.** Any type name is accepted; DBML does not restrict the type
   vocabulary. The type name MUST NOT contain spaces; a type containing
   spaces MUST be written as a quoted identifier (`"double precision"`,
   `"bigint unsigned"`). Parenthesized type arguments (`varchar(255)`,
   `decimal(10,2)`) are preserved verbatim. A type name may be
   schema-qualified to reference an enum (`v2.job_status`).
2. **Nullability.** Absent `not null`, a column is nullable. `null` and
   `not null` are mutually exclusive.
3. **`pk` / `primary key`** are synonyms and mutually exclusive; they mark a
   single-column primary key. A composite primary key MUST be expressed as
   an index with the `pk` setting (§6.5).
4. **`increment`** marks the column auto-increment.
5. **`check`** attaches a single-column check constraint; the setting MAY be
   repeated to attach multiple checks.
6. **`ref`** declares an inline relationship (§6.7). The setting MAY be
   repeated. Inline refs cannot carry names or settings.
7. **Legacy flags.** For backward compatibility, the bare words `pk` and
   `unique` MAY appear between the type and the settings list
   (`id int pk`). New documents SHOULD use the settings list instead.
8. Each column definition is terminated by a line break.

> **Extension (this implementation).** The column setting `tag: string`
> (repeatable) passes one Go struct tag through to the generated field
> verbatim ([decision D60](decisions.md)): every field property lives in
> the schema (D59), and the passthrough serves every encoding and
> third-party library without minting a setting name per format.
>
> 1. The value MUST be exactly one `key:"value"` pair in Go struct-tag
>    form: a key containing no spaces, colons, double quotes or
>    backticks, then a colon, then a double-quoted value containing no
>    double quotes or backticks.
> 2. Within one column, tag keys MUST be unique across all `tag`
>    settings.
> 3. The key `db` is reserved and an error — the `db` tag is the scan
>    contract (Appendix A.5).
> 4. A `json` tag replaces the generated default; every other tag is
>    appended after the generated pair, in declaration order
>    (Appendix A.5).
>
> The setting is accepted by this front end but is not upstream DBML.
> Note that `encoding/gob` ignores struct tags entirely: renaming a
> field for gob is a Go-field-name concern, not a tag concern, and has
> no surface here.

```volt
Table users {
  id        integer [pk]
  user_name varchar [not null, tag: 'json:"userName"', tag: 'xml:"user-name,attr"']
}
```

### 6.4 Default Values

```ebnf
default value = [ "-" ], number
              | string
              | boolean
              | null
              | expression literal
              | enum constant ;
```

| Kind          | Example                                    |
|---------------|--------------------------------------------|
| number        | `default: 123`, `default: -100`            |
| string        | `default: 'direct'`                        |
| boolean       | `default: false`, `default: null`          |
| expression    | `` default: `now() - interval '5 days'` `` |
| enum constant | `default: status.active`                   |

A bare identifier that is not `true`/`false`/`null` and not a dotted enum
constant is **not** a valid default value.

### 6.5 Indexes

An `indexes` block may appear inside a `Table` or `TablePartial` body. A
body MAY contain more than one `indexes` block; their contents accumulate.

```ebnf
indexes block  = "indexes", "{", { index }, "}" ;
index          = index key, [ index settings ], newline ;

index key      = index atom
               | "(", index atom, { ",", index atom }, ")" ;
index atom     = name | expression literal ;

index settings = "[", index setting, { ",", index setting }, "]" ;
index setting  = "type", ":", identifier
               | "name", ":", string
               | "unique"
               | "pk"
               | "note", ":", string ;
```

1. An index key is a single column, a single expression, or a parenthesized
   composite of columns and/or expressions.
2. Every `name` in an index key MUST refer to a column of the enclosing
   table (after partial injection).
3. `pk` declares the index a (composite) primary key; `unique` a unique
   index. Combining `pk` and `unique` on one index is redundant but
   permitted.
4. `type` selects the index method. Any identifier is accepted; the
   conventional, portable values are `btree` and `hash`.

```volt
Table bookings {
  id integer
  country varchar
  booking_date date

  indexes {
    (id, country) [pk]
    booking_date [name: 'idx_booking_date', type: hash]
    (country, `lower(country)`) [unique]
  }
}
```

### 6.6 Checks

A `checks` block declares table-level check constraints (constraints over one
or many columns). It may appear inside a `Table` or `TablePartial` body. A
body MAY contain more than one `checks` block; their contents accumulate.

```ebnf
checks block   = "checks", "{", { check }, "}" ;
check          = expression literal, [ check settings ], newline ;

check settings = "[", check setting, { ",", check setting }, "]" ;
check setting  = "name", ":", string ;
```

1. The expression is opaque SQL (§3.12); DBML does not parse or validate it.
2. `name` names the generated constraint.
3. Single-column checks MAY alternatively be written as a `check:` column
   setting (§6.3).

```volt
Table users {
  wealth integer
  debt integer

  checks {
    `debt + wealth >= 0` [name: 'chk_positive_money']
  }
}
```

### 6.7 Relationships (Ref)

Relationships define foreign-key constraints. There are three syntactic
forms: **long**, **short**, and **inline**.

```ebnf
ref element      = ref long | ref short ;

ref long         = "Ref", [ name ], "{", ref body, "}" ;
ref short        = "Ref", [ name ], ":", ref body ;

ref body         = ref endpoint, rel op, ref endpoint, [ ref settings ] ;

ref endpoint     = table name, ".", column group ;
column group     = name
                 | "(", name, { ",", name }, ")" ;

rel op           = "<>" | "<" | ">" | "-" ;

inline ref value = rel op, ref endpoint ;
                   (* value of a column's ref: setting *)

ref settings     = "[", ref setting, { ",", ref setting }, "]" ;
ref setting      = "delete", ":", ref action
                 | "update", ":", ref action
                 | "color", ":", color
                 | "inactive" ;
ref action       = "cascade" | "restrict" | "set null"
                 | "set default" | "no action" ;
```

1. **Cardinality operators** (left endpoint *op* right endpoint):
   - `<` one-to-many: `users.id < posts.user_id`
   - `>` many-to-one: `posts.user_id > users.id`
   - `-` one-to-one: `users.id - user_infos.user_id`
   - `<>` many-to-many: `authors.id <> books.id`
2. **Endpoints.** Each endpoint is a (optionally schema-qualified) table
   name — or table alias — followed by one column or a parenthesized column
   group. In a composite relationship both endpoints MUST list the same
   number of columns: `Ref: a.(x, y) > b.(x, y)`.
3. **Foreign-key side.** For `>`, the left endpoint is the foreign key. For
   `<`, the right endpoint is the foreign key. For `-`, the *second* (right)
   endpoint is the foreign key. For an inline ref, the declaring column is
   the foreign key.
4. **Inline form.** `ref: <op> <endpoint>` appears as a column setting
   (§6.3); the declaring column is the implicit left endpoint. Inline refs
   MUST NOT carry a relationship name or settings.
5. **Referential actions.** `delete:` and `update:` correspond to SQL
   `ON DELETE` / `ON UPDATE`.
6. `inactive` and `color` are visualization-only settings (rendered as a
   dotted line / line color); they have no SQL semantics.
7. A many-to-many (`<>`) relationship denotes an implicit junction table on
   SQL export; it MAY equivalently be modeled explicitly with two
   many-to-one relationships.
8. Zero-or-one / zero-or-many cardinality is not written explicitly; it is
   derived from the nullability of the foreign-key column (§6.3.2).
9. Duplicate relationships between the same column sets are invalid,
   regardless of direction: `a.x > b.y` and `b.y < a.x` are the same
   relationship.
10. A `Ref` element declares exactly **one** relationship. The long form is
    a stylistic variant of the short form, not a container for several
    relationships; to declare several, write several `Ref` elements.

```volt
// short form, with settings
Ref fk_posts_user: posts.user_id > core.users.id [delete: cascade, update: no action]

// long form
Ref {
  merchant_periods.(merchant_id, country_code) > merchants.(id, country_code)
}

// inline form
Table posts {
  user_id integer [ref: > core.users.id]
}
```

### 6.8 Enum

```ebnf
enum          = "Enum", table name, "{", { enum value }, "}" ;
enum value    = name, [ enum settings ], newline ;

enum settings = "[", enum setting, { ",", enum setting }, "]" ;
enum setting  = "note", ":", string ;
```

1. The enum name MAY be schema-qualified; unqualified enums belong to
   `public` (§8.1).
2. An enum MUST contain at least one value. Values MUST be unique within
   the enum.
3. A value containing spaces or special characters MUST be written as a
   quoted identifier: `"Not Yet Set"`.
4. A column references an enum by using the (optionally schema-qualified)
   enum name as its type: `status v2.job_status`.

```volt
Enum job_status {
  created [note: 'Waiting to be processed']
  running
  done
  failure
}
```

### 6.9 TablePartial

A `TablePartial` declares a reusable fragment of a table body. Tables inject
partials by name.

```ebnf
table partial     = "TablePartial", name, [ table settings ],
                    "{", partial body, "}" ;
partial body      = { column
                    | indexes block
                    | checks block
                    | note def
                    } ;

partial injection = "~", name, newline ;
```

1. Partial names live in their own global (schema-less) namespace and MUST
   be unique.
2. A partial body is a table body without `records` and without nested
   partial injections.
3. An injection `~p` inside a table body replaces itself with the entire
   content of partial `p` at that position. Injection order is source
   order.
4. **Conflict resolution** when the same column, setting, or index is
   defined more than once:
   1. A definition written directly in the table overrides any partial.
   2. Otherwise the **last-injected** partial (in source order) wins.

```volt
TablePartial base_template [headercolor: #ff0000] {
  id int [pk, not null]
  created_at timestamp [default: `now()`]
}

Table users {
  ~base_template
  name varchar
}
```

### 6.10 Records (Sample Data)

`Records` declares sample rows for a table, in CSV-like syntax. Records may
be declared at top level (naming the table) or inside a table body.

```ebnf
records element = "Records", table name, records columns,
                  "{", { record row }, "}" ;

records block   = "Records", [ records columns ],
                  "{", { record row }, "}" ;

records columns = "(", name, { ",", name }, ")" ;

record row      = record value, { ",", record value }, newline ;
record value    = string
                | [ "-" ], number
                | boolean
                | null
                | expression literal
                | enum constant          (* EnumName.value, §4.1 *)
                | empty ;
empty           = ;                      (* nothing between separators *)
```

1. Top-level records MUST name the target table and MUST list columns.
   In-table records MAY omit the column list; the columns then default to
   all table columns in definition order (after partial injection, §8.4).
2. A table MUST have at most one records block.
3. Each row MUST supply exactly as many values as listed columns.
4. **Value typing.** Each value is checked against the target column's type:
   - Strings are single-quoted. Timestamps/dates are strings in ISO 8601 or
     other unambiguous formats (`'2024-01-15 10:30:00'`, `'2024-01-15'`).
   - Booleans accept, case-insensitively: `true`, `false`, `'true'`,
     `'false'`, `'Y'`, `'N'`, `'T'`, `'F'`, `1`, `0`, `'1'`, `'0'`.
   - Null is `null`, an empty field (nothing between commas), or the empty
     string `''` for non-string columns.
   - Enum values are written as `EnumName.value` or as a string literal of
     the value. A bare identifier (other than `true`/`false`/`null`) is
     **not** a valid record value.
   - Expression literals (backticks) pass through unchecked.

```volt
Table users {
  id int [pk]
  name varchar
  status job_status

  Records (id, name, status) {
    1, 'Alice', job_status.created
    2, 'Bob', 'running'
    3, , null
  }
}

Records users(id, name, status) {
  4, 'Carol', `default_status()`
}
```

### 6.11 Notes

Notes attach human-readable documentation. There are two positions: a
**note definition** inside an element body, and a top-level **sticky note**
element.

```ebnf
note def      = "Note", ":", string, newline
              | "Note", "{", string, "}" ;

sticky note   = "Note", name, [ note settings ], "{", string, "}" ;

note settings = "[", "color", ":", ( color | "none" ), "]" ;
```

1. A `note def` may appear in the body of `Project`, `Table`,
   `TablePartial`, and `TableGroup`. `Table`, `TablePartial`, and
   `TableGroup` bodies MUST contain at most one note def; uniqueness is
   not enforced in `Project` bodies.
2. Notes on columns, indexes, and enum values use the `note:` setting
   instead (§6.3, §6.5, §6.8).
3. The value is a string; multi-line strings (§3.7) are permitted and
   conventionally contain Markdown.
4. A sticky note is a named, free-standing note (visualization only). Its
   `color` setting accepts a color literal or `none` (no background).

```volt
Note deployment_reminder [color: #F4D03F] {
  'Remember to run migrations after deploy'
}
```

### 6.12 TableGroup

Groups related tables (documentation/visualization only; no SQL semantics).

```ebnf
table group          = "TableGroup", name, [ table group settings ],
                       "{", table group body, "}" ;
table group body     = { ( table name, newline ) | note def } ;

table group settings = "[", table group setting,
                       { ",", table group setting }, "]" ;
table group setting  = "note", ":", string
                     | "color", ":", color ;
```

1. Each body line names one table (optionally schema-qualified, or an
   alias). Every named table MUST exist.
2. A table MUST NOT belong to more than one TableGroup.

```volt
TableGroup e_commerce [color: #3498DB, note: 'Core commerce tables'] {
  merchants
  countries
}
```

### 6.13 DiagramView

Declares a named view of the diagram, selecting which items are shown
(visualization only).

```ebnf
diagram view  = "DiagramView", name, "{", { view category }, "}" ;

view category = category kind, "{", category body, "}" ;
category kind = "Tables" | "Notes" | "TableGroups" | "Schemas" ;
category body = "*"
              | { table name, newline } ;
```

1. Each category may appear at most once per view.
2. `*` selects all items of that category; otherwise items are listed one
   per line. An empty body (or omitted category) selects nothing.
3. Listed names MUST refer to existing elements of the corresponding kind.

```volt
DiagramView sales_view {
  Tables { users
           orders }
  Schemas { core }
  Notes { * }
}
```

---

## 7. File Imports (removed)

DBML's file-based module system — `use * from './file'`, selective
import with per-element kinds and `as` aliases, and `reuse` re-export —
**is not part of the Volt language**. Splitting a schema across files
needs no imports at all: a package is a directory and every file in it
shares one namespace (§V1); cross-package references go through the
package system (§V2). The rationale and the mechanical migration are
Appendix C.

A conforming implementation recognizes the historical forms in order to
reject them precisely:

```ebnf
import statement = import kw, import spec, "from", import path ;
import kw        = "use" | "reuse" ;
import spec      = "*" | "{", import item, newline,
                   { import item, newline }, "}" ;
import item      = element kind, table name, [ "as", name ] ;
import path      = string ;
```

1. An `import statement` anywhere in a file is an **error** (code
   `spec/7`), at every layer: the single-file schema pass and the
   project pass alike. The diagnostic points at Appendix C.
2. No other behavior attaches to the statement: it declares nothing,
   imports nothing, and suppresses only cascading unresolved-name
   noise within its file.

---

## 8. Static Semantics

### 8.1 Schemas

1. Schemas are not declared; a schema exists if and only if at least one
   table or enum names it as qualifier.
2. Every unqualified table, enum, or relationship endpoint belongs to the
   default schema **`public`**.

### 8.2 Namespaces and Uniqueness

Within one compiled schema (after imports):

1. Table names (and aliases) MUST be unique per schema; the alias namespace
   is shared with unqualified table names.
2. Enum names MUST be unique per schema.
3. TablePartial names, TableGroup names, sticky-note names, and DiagramView
   names each form a single global namespace and MUST be unique.
4. Column names MUST be unique within their table; enum values within their
   enum.

### 8.3 Reference Resolution

1. Relationship endpoints, index columns, records columns, TableGroup
   members, and DiagramView members MUST resolve to existing elements.
2. A table alias may be used interchangeably with the table name in
   endpoints and group members.
3. Composite relationship endpoints MUST have equal arity, and referenced
   column lists must match in count and order.

### 8.4 Partial Injection Order

1. Injections are expanded in source order; the effective column order of a
   table is the concatenation of injected and direct columns in source
   order (this order also drives implicit records column lists, §6.10).
2. Conflicts resolve per §6.9.4.

### 8.5 Nullability and Cardinality

1. A column without `null`/`not null` is nullable.
2. A nullable foreign-key column yields zero-or-one / zero-or-many
   cardinality on the FK side; `not null` yields exactly-one.

---

## Appendix IA: Collected Grammar (Part I)

The complete grammar in EBNF (ISO/IEC 14977), collected from the sections
above.

```ebnf
(* ===== 5. Program ===== *)

program              = { import statement | element } ;

element              = project | table | table partial | enum
                     | ref element | sticky note | table group
                     | records element | diagram view ;

(* ===== 7. Module system ===== *)

import statement     = import kw, import spec, "from", import path ;
import kw            = "use" | "reuse" ;
import spec          = "*"
                     | "{", import item, newline,
                       { import item, newline }, "}" ;
import item          = element kind, table name, [ "as", name ] ;
element kind         = "table" | "enum" | "tablepartial" | "note"
                     | "schema" | "tablegroup" ;
import path          = string ;

(* ===== 6.1 Project ===== *)

project              = "Project", [ name ], "{", project body, "}" ;
project body         = { project property | note def } ;
project property     = identifier, ":", string, newline ;

(* ===== 6.2–6.6 Table ===== *)

table                = "Table", table name, [ table alias ],
                       [ table settings ], "{", table body, "}" ;
table alias          = "as", name ;
table body           = { column | indexes block | checks block
                       | note def | partial injection | records block } ;
table settings       = "[", table setting, { ",", table setting }, "]" ;
table setting        = "headercolor", ":", color
                     | "note", ":", string ;

column               = name, column type, { legacy flag },
                       [ column settings ], newline ;
column type          = type name, [ "(", type arg, { ",", type arg }, ")" ] ;
type name            = [ schema name, "." ], identifier ;
type arg             = number | identifier ;
legacy flag          = "pk" | "unique" ;
column settings      = "[", column setting, { ",", column setting }, "]" ;
column setting       = "primary key" | "pk" | "null" | "not null"
                     | "unique" | "increment"
                     | "default", ":", default value
                     | "check", ":", expression literal
                     | "note", ":", string
                     | "ref", ":", inline ref value ;
default value        = [ "-" ], number | string | boolean | null
                     | expression literal | enum constant ;

indexes block        = "indexes", "{", { index }, "}" ;
index                = index key, [ index settings ], newline ;
index key            = index atom
                     | "(", index atom, { ",", index atom }, ")" ;
index atom           = name | expression literal ;
index settings       = "[", index setting, { ",", index setting }, "]" ;
index setting        = "type", ":", identifier
                     | "name", ":", string
                     | "unique" | "pk"
                     | "note", ":", string ;

checks block         = "checks", "{", { check }, "}" ;
check                = expression literal, [ check settings ], newline ;
check settings       = "[", check setting, { ",", check setting }, "]" ;
check setting        = "name", ":", string ;

(* ===== 6.7 Ref ===== *)

ref element          = ref long | ref short ;
ref long             = "Ref", [ name ], "{", ref body, "}" ;
ref short            = "Ref", [ name ], ":", ref body ;
ref body             = ref endpoint, rel op, ref endpoint, [ ref settings ] ;
ref endpoint         = table name, ".", column group ;
column group         = name | "(", name, { ",", name }, ")" ;
rel op               = "<>" | "<" | ">" | "-" ;
inline ref value     = rel op, ref endpoint ;
ref settings         = "[", ref setting, { ",", ref setting }, "]" ;
ref setting          = "delete", ":", ref action
                     | "update", ":", ref action
                     | "color", ":", color
                     | "inactive" ;
ref action           = "cascade" | "restrict" | "set null" | "set default"
                     | "no action" ;

(* ===== 6.8 Enum ===== *)

enum                 = "Enum", table name, "{", { enum value }, "}" ;
enum value           = name, [ enum settings ], newline ;
enum settings        = "[", enum setting, { ",", enum setting }, "]" ;
enum setting         = "note", ":", string ;

(* ===== 6.9 TablePartial ===== *)

table partial        = "TablePartial", name, [ table settings ],
                       "{", partial body, "}" ;
partial body         = { column | indexes block | checks block | note def } ;
partial injection    = "~", name, newline ;

(* ===== 6.10 Records ===== *)

records element      = "Records", table name, records columns,
                       "{", { record row }, "}" ;
records block        = "Records", [ records columns ],
                       "{", { record row }, "}" ;
records columns      = "(", name, { ",", name }, ")" ;
record row           = record value, { ",", record value }, newline ;
record value         = string | [ "-" ], number | boolean | null
                     | expression literal | enum constant | empty ;
empty                = ;

(* ===== 6.11 Notes ===== *)

note def             = "Note", ":", string, newline
                     | "Note", "{", string, "}" ;
sticky note          = "Note", name, [ note settings ], "{", string, "}" ;
note settings        = "[", "color", ":", ( color | "none" ), "]" ;

(* ===== 6.12 TableGroup ===== *)

table group          = "TableGroup", name, [ table group settings ],
                       "{", table group body, "}" ;
table group body     = { ( table name, newline ) | note def } ;
table group settings = "[", table group setting,
                       { ",", table group setting }, "]" ;
table group setting  = "note", ":", string
                     | "color", ":", color ;

(* ===== 6.13 DiagramView ===== *)

diagram view         = "DiagramView", name, "{", { view category }, "}" ;
view category        = category kind, "{", category body, "}" ;
category kind        = "Tables" | "Notes" | "TableGroups" | "Schemas" ;
category body        = "*" | { table name, newline } ;

(* ===== 4. Common forms ===== *)

name                 = identifier ;
schema name          = name ;
table name           = [ schema name, "." ], name ;
column path          = [ schema name, "." ], name, ".", name ;
enum constant        = name, ".", name ;

settings             = "[", setting, { ",", setting }, "]" ;
setting              = setting name, [ ":", setting value ] ;
setting name         = identifier, { sp, { sp }, identifier } ;
setting value        = string | number | boolean | null | color
                     | expression literal | identifier
                     | inline ref value ;

(* ===== 3. Lexical grammar ===== *)

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
                     | digit, { ident char } ;
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
                     | any char ;
hex digit            = digit
                     | "a" | "b" | "c" | "d" | "e" | "f"
                     | "A" | "B" | "C" | "D" | "E" | "F" ;

number               = digit, { digit },
                       [ ".", digit, { digit } ], [ exponent ] ;
exponent             = ( "e" | "E" ), [ "+" | "-" ], digit, { digit } ;

boolean              = "true" | "false" ;
null                 = "null" ;

color                = "#", ( 3 * hex digit | 6 * hex digit ) ;

expression literal   = "`", { any char - "`" }, "`" ;
```

---

---

# Part II — The Project and Routing Layer (§V)

**Version:** 0.1 (v0 surface: packages, imports, pipelines, scopes,
routes, resources)
**Status:** Normative for the v0 implementation in this repository.
Part I specifies the schema core; this part specifies everything Volt
adds above it. Section numbers here are prefixed **§V** so the two
parts cross-reference without collision; diagnostics cite the section
they enforce (`spec/V4`).

Every construct below is specified by (1) a grammar production in EBNF
(notation of Part I §1), (2) an enumerated list of constraints, and
(3) a minimal example. The collected grammar appears in
[Appendix VA](#appendix-va-collected-grammar). The executable companion
is the conformance corpus in [`lang/conformance/snippets/`](../lang/conformance/snippets/):
files under `valid/` MUST be accepted by a conforming implementation,
files under `invalid/` MUST be rejected.

---

## §V0. Layers, files and the superset rule

1. A Volt source file conventionally uses the extension **`.volt`**.
   Whether a file happens to use only the DBML core (Part I) or the
   full language follows from its content, never from its name.
2. **Superset rule.** Every valid DBML program whose declarations avoid
   the import statements of Part I §7 is a valid Volt file body
   (§V2.5 removes `use`/`reuse`; Appendix C covers migration). The
   lexical grammar is Part I §3 with one addition:

   ```ebnf
   slash = "/" ;
   ```

   `/` is a token wherever it is not the start of a `//` or `/*`
   comment. Consequence: **`/*` always opens a block comment**, which
   is why the rest-of-path wildcard is spelled `:name...` (§V4.1.4)
   and not `*name` — after a slash, a star-marked wildcard is lexically
   unreachable in a superset of DBML.
3. Declaration order and file assignment carry no semantics (§V1.5).

## §V1. Projects and packages

```ebnf
package clause = "package", name, newline ;
```

1. A **project** is a directory tree rooted at the nearest ancestor
   directory containing a file named **`volt.mod`**. `volt.mod`
   contains comments (`//`), blank lines, and exactly one directive:
   `module <name>`.
2. Every `.volt` file MUST begin with a package clause: its first
   declaration, at most one per file. The name is a plain (unquoted)
   identifier. The name `volt` is reserved: generated files import the
   volt runtime under that qualifier (§V7), so a package of that name
   is an error.
3. A **package** is a directory: all `.volt` files in one directory
   MUST declare the same package name, and share one namespace per
   element kind (tables, enums, partials, pipelines, scopes — each per
   Part I §8.2 extended with §V3.1 and §V4).
4. In every directory other than the project root, the package name
   MUST equal the directory's base name. The root directory's package
   may take any name.
5. **Layout invariance.** Splitting or merging the `.volt` files of a
   package, or reordering declarations, MUST NOT change the meaning of
   the package. Implementations MUST process a package as the
   concatenation of its files in file-name order, and route expansion
   (§V4.7) follows declaration order within that concatenation.
6. Directories whose name begins with `.` or `_`, and directories named
   `node_modules`, are not part of the project. Neither is any
   subdirectory containing its own `volt.mod` (a nested project's
   packages belong to it, not to the enclosing module).

```volt
// db/schema.volt
package db

Table users {
	id integer [pk, increment]
}
```

## §V2. Imports

```ebnf
import decl = "import", "(", { import spec }, ")" ;
import spec = [ alias ], import path, newline ;
alias       = name ;
import path = name, { slash, name } ;
```

1. An import declaration is a parenthesized, newline-separated block of
   import specs; an empty block is an error. Import declarations appear
   after the package clause, before other use of the imported names.
2. An import path names a package by its directory path from the
   project root, `/`-separated, with no `./`, no `..`, and no interior
   whitespace; its segments, and the alias, are plain (unquoted)
   identifiers (§V4.1.6). The path MUST name an existing package of
   the project.
3. The imported package is referenced through its **qualifier**: the
   alias when given, else the last path segment. All cross-package
   references are qualifier-prefixed (`db.User`); nothing is ever
   imported into the local scope.
4. Within one package (across all its files): importing the same path
   twice is idempotent, but with two different qualifiers it is an
   error; two different paths MUST NOT share a qualifier; a package
   MUST NOT import itself; **every import MUST be used** — an import
   whose qualifier is never referenced is an error.
5. **DBML file imports are removed.** The `use` and `reuse` statements
   of Part I §7 are not part of the Volt language at any layer; a
   conforming implementation rejects them with a migration diagnostic.
   (The parser MAY still recognize the syntax so the DBML conformance
   corpus remains checkable; acceptance is what is forbidden.)
6. **Import cycles are errors.** The package import graph MUST be
   acyclic.

```volt
package app

import (
	db
	d2 shared/dicts
)
```

## §V3. Pipelines

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
   not supported in v0 and is an error suggesting a local wrapper.
   Whether the referenced Go function exists is the Go compiler's
   business, not the checker's.
3. Pipelines contribute middleware in declaration order, outermost
   first, composed statically in generated code (§V4.4).

```volt
Pipeline api {
	use volt.RequestID
	use BearerAuth
}
```

## §V4. Scopes and routes

### §V4.1 Route paths

```ebnf
route path = slash,
             [ segment, { slash, segment } ] ;
segment    = name
           | ":", name, [ "(", type name, ")" ]
           | ":", name, "...", "...", "..." ;  (* three '.' tokens *)
type name  = "int" | "int32" | "int64" | "string" ;
```

1. All tokens of a route path are **contiguous**: interior whitespace
   ends the path. A bare `/` is the root path. A non-root path MUST NOT
   end with `/` and MUST NOT contain an empty segment (`//`): routes
   match exactly (§V4.4), so a trailing slash would spell a silently
   different pattern.
2. A `:name` segment captures one path segment as a parameter. The
   parameter name MUST be a valid, non-keyword Go identifier, MUST NOT
   be one of the reserved names `w`, `r`, `opts`, `volt` (they appear
   in generated signatures and would be shadowed), and MUST be unique
   within the route's full path (scope prefixes included).
3. A parameter's type defaults to `string`; the closed type set is
   `int`, `int32`, `int64`, `string`, chosen to coincide with the Go
   types the model generator emits for routable primary keys.
   **Types shape handler signatures, never matching**: a request
   segment that fails to parse as the declared type is that route's
   404, not a fallthrough to a sibling route.
4. `:name...` captures the rest of the path (a **wildcard**), always as
   `string`. It MUST be the last segment of the full path, MUST NOT
   carry a type, and MUST NOT appear in a Scope prefix. (Spelling
   rationale: §V0.2.)
5. Grammar note: a path is assembled from ordinary tokens; the
   contiguity rule of clause 1 is what makes it one lexical island.
6. Every segment name — literal, parameter or wildcard — MUST be a
   plain (unquoted) identifier of ASCII letters, digits and
   underscores. Segment names flow verbatim into registration patterns
   and generated Go; the closed character set is what makes that safe
   (and is why `{`/`}` can serve as the collision-free conflict markers
   of §V4.7.1). The same constraint applies to the `resources` table
   name (§V5.1), to the identifiers of pipeline names and plug
   references (§V3), to route handler names (§V4.3), and to import
   aliases and path segments (§V2.2).

### §V4.2 Routes

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

### §V4.3 Handler references

```ebnf
handler ref = name, ".", name ;
```

1. A handler is `Controller.Action`: exactly two exported Go
   identifiers. The controller MUST NOT be an import qualifier —
   handlers live in the routes package.
2. Every distinct controller becomes one generated interface; every
   distinct action one method with the route's typed parameters
   appended after `(w http.ResponseWriter, r *volt.Request)`, returning
   `error`.
3. Two routes MAY share `Controller.Action` only with identical
   parameter signatures (names, types and wildcard-ness, in order).

### §V4.4 Scopes

```ebnf
scope      = "Scope", route path, [ settings ], "{", { scope item }, "}" ;
scope item = route | resources | scope ;
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
3. The `error_handler` function has the volt runtime's ErrorHandler
   shape: `func(http.ResponseWriter, *volt.Request, error)`. Routes
   without one use the runtime default.

### §V4.5 (reserved)

### §V4.6 Route names and reverse URLs

1. Every route derives a **helper name**: the scope name prefixes
   followed by the `[name:]` setting (normalized to a Go name) when
   present, else the action name. Resources derive per §V5.4.
2. Each named route yields one generated function
   `Path<Helper>(typed params..., opts ...volt.URLOption) string` that
   produces exactly the paths its route matches. Helper names form one
   namespace per package; a collision is an error. Helper output is
   always a clean path: string parameters are percent-escaped, and the
   values `.` and `..` escape entirely (`%2E` forms) — as literal
   segments they would change the path's shape under cleaning.

### §V4.7 Route conflicts

1. A route's **shape** is its full path with every parameter replaced
   by `{}` and every wildcard by `{...}`; literals compare by spelling.
   (Braces cannot appear in a literal segment, §V4.1.6, so the markers
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
3. This is exactly the rule `http.ServeMux` enforces by panicking at
   registration time. Detection therefore happens at check time, so a
   checked package always registers cleanly; the registration panic
   remains as a backstop a conforming generator never triggers.

```volt
Scope / [pipe: api, error_handler: Errors] {
	get /            Home.Index [name: root]
	Scope /admin [name: admin] {
		get /stats   Admin.Stats
	}
	get /files/:path...           Files.Serve
	get /users/:id(int32)/avatar  Users.Avatar
}
```

## §V5. Resources

```ebnf
resources = "resources", table ref, [ settings ], newline ;
table ref = name, [ ".", name ] ;
```

### §V5.1 Declaration

1. `resources <table>` appears only inside a Scope body and expands to
   the action routes of §V5.2 with the table name as the collection
   segment. The name MUST map to a Go identifier.
3. The declaration MUST resolve to a declared table (§V5.4); there
   is no schemaless form. A miss — including a name that only matches
   a table's model name or differs only in case — is an error naming
   the correct spelling where one exists.

### §V5.2 The action table

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

### §V5.3 Settings

| Setting | Value | Meaning |
|---|---|---|
| `api` | flag | restrict to index, create, show, update, delete |
| `only` | action list `(index, show)` | keep only these actions |
| `except` | action list | drop these actions |
| `param` | identifier | key parameter name (default `id`) |
| `singular` | identifier | the singular used for member helper names (§V5.4) |

1. Action names in `only`/`except` are the lowercase names of §V5.2;
   unknown names are errors. `only` and `except` MUST NOT be combined.
   `only`/`except` filter the action set after `api`.
2. The key parameter's type always comes from the resolved table's
   primary key (§V5.4.3).
3. `singular` overrides the inflector of §V5.4.2. It is required
   whenever singularization leaves the name unchanged, which would
   otherwise make the collection and member helpers collide.

### §V5.4 Table resolution

1. The declaration names a **table** of this package (`posts`) or of an
   imported package (`db.posts`); a qualified reference marks the
   import used (§V2.4).
2. The name matches the table's declared name **exactly**: names are
   case-sensitive, and a name differing only in case is an error naming
   the right spelling. A qualified name that matches nothing is an
   error; an unqualified one that matches nothing is a resource
   without a schema (clause 6).
3. The resolved table MUST have a single-column primary key whose
   Go type is `int`, `int32`, `int64` or `string`; that type
   becomes the key parameter's type. Composite, missing, or unroutable
   keys are errors.
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
   former is not a resources setting (§V6), the latter prefixes both
   sides equally.

```volt
resources db.users [only: (index, show, create)]
```

## §V6. Settings whitelists

Settings valid on Volt elements, exhaustively (a setting not listed for
an element is an error on that element):

| Element | Settings |
|---|---|
| Scope | `pipe`, `name`, `error_handler` |
| route | `name` |
| resources | `api`, `only`, `except`, `param`, `singular` |

The identifier-list value form `(a, b, c)` (production in Appendix VA)
is valid only where a setting explicitly takes an action list.

## §V7. Generation contract (informative)

The normative output contract is the golden corpus under
`gen/router/testdata/` and the proof suite under `itest/`. In prose:
one `<Controller>Controller` interface per controller and a
`Controllers` struct (§V4.3); `New(Controllers) http.Handler`
registering every route onto a `http.ServeMux` with its pipeline chain
composed statically and its typed shim parsing parameters per §V4.1.3;
`Path*` helpers per §V4.6; a `Table []volt.Route` mirroring the
expanded route list in declaration order. All generated files carry the
standard generated-code header and are gofmt-stable.

## §V8. Reserved words for future layers

`Dataset` is reserved: a conforming v0 implementation rejects it with a
forward-pointing diagnostic. (Design: [roadmap FW-2](roadmap.md).)

---

## §V9. Groups

A **Group** names a set of tables so the same code can be generated for
every member (§V11). It is not a diagram construct: `TableGroup`
(Part I §6.12) partitions the ER diagram and allows one group per
table; a `Group` is a query set — overlapping freely, invisible to
diagrams.

```ebnf
group decl = "Group", plain name,
             ( "{", { newline }, [ group members ], "}"
             | "=", group expr ), newline ;
group members = group term, { newline+, group term }, { newline } ;
group expr    = group term, { ( "+" | "-" ), group term } ;
group term    = plain name ;
```

```volt
Group series {
  ms_revenue
  ms_usage
  ks_seats
}

Group wide = series + ks_costs - ms_usage
```

1. Group names share one namespace per package, disjoint from tables;
   redeclaration is an error.
2. A `group term` resolves, case-sensitively, to a table or a group of
   the same package — tables first, then groups. An unknown name is an
   error with the same did-you-mean aids as §V5.4.
3. The block form is the expression form with every term joined by
   `+`. Evaluation is left to right: `+` adds a term's member set
   (a table adds itself), `-` removes it. Removing a table that is not
   currently a member, or adding one already present, is an error —
   the algebra must say something true.
4. Group references must be acyclic (error otherwise), and the
   resulting set must be non-empty.
5. Member order is first-addition order; generation (§V11) is
   deterministic in it.

## §V10. Predicates

A **Pred** names a boolean expression over the columns of an as-yet
unnamed table. The expression language is deliberately **closed** —
it is not SQL and never grows toward it (D06); anything it cannot say
belongs in a raw `Select` body (reserved) or the dynamic layer.

```ebnf
pred decl    = "Pred", plain name, "{", { newline }, pred expr, { newline }, "}", newline ;
pred expr    = pred and, { "or", pred and } ;
pred and     = pred unary, { "and", pred unary } ;
pred unary   = [ "not" ], pred primary ;
pred primary = "(", pred expr, ")"
             | comparison | membership | pattern | null test
             | plain name ;                      (* reference to a Pred *)
comparison   = operand, comp op, operand ;
comp op      = "=" | "!=" | "<" | "<=" | ">" | ">=" ;
membership   = column ref, "in", "(", literal, { ",", literal }, ")" ;
pattern      = column ref, "like", ( string | param ) ;
null test    = column ref, "is", [ "not" ], "null" ;
operand      = column ref | param | literal ;
column ref   = name ;
param        = ":", plain name ;                 (* no space after ':' *)
literal      = number | string | boolean ;
```

```volt
Pred current { org = :org and year = :year }
Pred recent  { year >= :since }
Pred fresh   { current and recent }
```

1. `and`, `or`, `not`, `in`, `like`, `is`, `null` are contextual
   keywords (§3.5), case-insensitive, not reserved.
2. A bare name in primary position references a Pred of the same
   package; references must exist and be acyclic.
3. A Pred is typed **at each use site** (§V11), where a target binds
   its column names. The rules:
   - a `column ref` must resolve in the target's environment (§V11.4);
   - `=` and `!=` require both operands to share a type class;
   - `<` `<=` `>` `>=` require numeric or date/time operands — text is
     **not** orderable here (`text_column > 1` and
     `text_column > 'a'` are both errors; order text in SQL, §V11.6);
   - `like` requires a text column and a text pattern;
   - `in` items must all match the column's type;
   - blob and JSON columns may not appear in predicates;
   - a `param` adopts the type of the expression position it appears
     in; one param name MUST resolve to one type across the whole use
     site, or the use is an error naming both positions.
4. Type classes are defined by the Appendix A mapping: two column
   types agree iff they map to the same Go type; numeric = the
   integer, unsigned and float families; text = `string`-mapped;
   date/time = `time.Time`-mapped. `decimal`/`money` map to `string`
   and are therefore **not** orderable in a predicate — deliberate.
5. Checks (Part I §6.6) are the parameterless ancestors of predicates;
   unifying them onto this language is planned, not yet specified.

## §V11. Selects over groups

A **Select** declares a query once and generates it for every member
of a target (§V9 group, or a single table treated as a one-member
group).

```ebnf
select decl = "Select", plain name, [ projection ], "for", plain name,
              [ "where", pred expr ], [ settings list ], newline ;

projection  = "(", column name, { ",", column name }, ")"
            | "(", "*", "-", column name, { "-", column name }, ")" ;
```

```volt
Select rows    for series where current [order: (year desc, id asc)]
Select stale   for ms_usage where not recent
Select all     for series
Select summary (id, org, year)    for series where current
Select public  (* - password_hash) for accounts
```

The clauses read in SQL order — name, columns, source, filter — and
the optional projection narrows the emitted columns.

1. The select name is a plain identifier; per target-member it mints
   the method `<Model><SelectName>` (Appendix A naming). A collision
   with a generated CRUD/dynamic method name, or between two selects
   on an overlapping member, is an error.
2. `for` resolves case-sensitively to a group, else a table (§V9.2
   aids apply).
3. `where` takes a full §V10 expression — named Preds, inline
   comparisons, and any composition of the two. Omitted = all rows.
4. **The agreement rule.** Every column referenced by the `where`
   expression (Pred references expanded) and by `order:` MUST exist in
   **every** member, and all members MUST agree on its type (§V10.4).
   A missing column or a type disagreement is an error naming the
   offending members and their types — the predicate must be checkable
   for all members or it does not compile.
5. Settings: `order:` takes a parenthesized list of `column asc` /
   `column desc` pairs. The direction is **mandatory** — SQL's silent
   `asc` default is deliberately not inherited; explicit over implicit
   — and the emitted SQL spells it out (`ORDER BY year DESC, id ASC`).
   Order columns obey rule 4 and MUST be orderable (§V10.3) or text —
   ORDER BY text is SQL's own collation, allowed here.
6. Generation contract (with Appendix A): per member, a method on
   `Queries` —
   `func (q *Queries) <Model><SelectName>(ctx context.Context, <params>) ([]<Row>, error)`
   — parameters in first-appearance order over the expanded
   expression, each typed by rule §V10.3; `<Row>` is the member's row
   type per rule 7 (the model itself when there is no projection); the
   emitted SQL is
   `SELECT <projected columns> FROM <table> [WHERE …] [ORDER BY …]`
   with SQLite named parameters (D15), and every emitted statement is
   prepare-validated against the generated DDL (D06).
7. **Projection and row types.** Without a projection the row type is
   the member's model and the emitted columns are all of it. With one:
   - **Explicit list** `(a, b, c)` — at least one column, no
     duplicates. Every listed column MUST exist in every member with
     one agreed generated **field type** (Appendix A — nullability
     included, so `text` and `text [not null]` disagree); a miss or a
     disagreement is an error naming the members and their types. The
     row type is **one shared struct** named by the select name
     mapped per Appendix A.3 (select `summary` → `Summary`), serving
     every member: one wire type, N sources. Its fields keep the
     columns' declared order from the list, each with the agreed type
     and the **default** tag pair `db:"col" json:"col"` (A.5) — the
     shared type belongs to the select, not to any one table, so
     per-table `tag:` passthroughs and doc comments do not transfer.
     Agreement is judged on the generated field type, so an enum-typed
     column agrees exactly when every member resolves it to the same
     generated enum type.
   - **Star with exclusions** `(* - a - b)` — at least one exclusion,
     no duplicates. Every excluded column MUST exist in every member
     (§V9.3: the algebra must say something true), and no member may
     end up with zero columns. Each member projects its own columns,
     in declaration order, minus the exclusions, minting a per-member
     **struct derivative** named `<Model><SelectName>`
     (`User` + `public` → `UserPublic`): every kept field is copied
     verbatim from the model — Go name, Go type, assembled struct tag
     (A.5, `tag:` passthroughs included), doc comment — so the
     derivative behaves exactly like the model minus the removed
     fields.
   - Columns referenced by `where` or `order:` need not be projected.
   - Minted row-type names (the shared struct, and each derivative)
     live in the generated package scope and MUST NOT collide with any
     generated package-level name. The checker reports collisions with
     models, enums, the `Queries` handle, another select's minted row
     type, the generated params types (`CreateParams`/`UpdateParams`
     select names are rejected outright), and the member tables' own
     dynamic column handles; the Go compiler backstops the rest.

---

## Appendix VA: Collected grammar

Additions to the collected grammar of Part I, Appendix IA. The
`element` production is extended:

```ebnf
element        = (* Part I Appendix IA alternatives *)
               | package clause | import decl | pipeline | scope
               | group decl | pred decl | select decl ;

slash          = "/" ;

package clause = "package", name, newline ;

import decl    = "import", "(", { import spec }, ")" ;
import spec    = [ alias ], import path, newline ;
alias          = name ;
import path    = name, { slash, name } ;

pipeline       = "Pipeline", name, "{", { plug }, "}" ;
plug           = "use", go ref, newline ;
go ref         = name, [ ".", name ] ;

scope          = "Scope", route path, [ settings ],
                 "{", { scope item }, "}" ;
scope item     = route | resources | scope ;

route          = verb, route path, handler ref, [ settings ], newline ;
verb           = "get" | "post" | "put" | "patch" | "delete"
               | "options" | "head" | "any" ;
handler ref    = name, ".", name ;

route path     = slash, [ segment, { slash, segment } ] ;
segment        = name
               | ":", name, [ "(", type name, ")" ]
               | ":", name, "." , ".", "." ;
type name      = "int" | "int32" | "int64" | "string" ;

resources      = "resources", name, [ settings ], newline ;

(* setting value, extended (Part I §4.2): *)
setting value  = (* schema-layer alternatives *) | ident list ;
ident list     = "(", name, { ",", name }, ")" ;
```

The `group decl`, `pred decl` and `select decl` productions — with the
predicate expression grammar they share — are collected in §V9, §V10
and §V11; they are not duplicated here.

All tokens of a `route path` and of an `import path` MUST be contiguous
(§V4.1.1, §V2.2) — the grammar above is subject to that adjacency
constraint, which the token stream expresses via inter-token whitespace
flags.

---

## Conformance and the proof chain

Testing never proves software correct; a specification with an
executable conformance surface narrows the gap deliberately. The v0
chain, each link runnable by `go test ./...`:

1. **Corpus ↔ spec.** Every file in `lang/conformance/snippets/`
   carries a `// spec: §V…` tag; `valid/` MUST check clean, `invalid/`
   MUST be rejected. The corpus is the spec's executable surface.
2. **Schema layer preserved.** The schema conformance corpus continues to run
   unchanged against the same front end: the superset rule (§V0.2) is
   enforced, not assumed.
3. **Generator ↔ contract.** Goldens are byte-compared, gofmt-stable by
   test, and **compiled by the real Go toolchain** against stub
   implementations of the generated interfaces — the typed-signature
   contract (§V4.3) is checked by the Go compiler itself.
4. **Router semantics.** The `itest/` fixture app is generated,
   committed, drift-checked against regeneration, and exercised over
   httptest: match and 404/405 behavior, typed-parameter 404s (§V4.1.3),
   pipeline order (§V4.4), the error spine (§V4.4.3), and
   **reverse-URL round-trip totality** — for every route in the
   generated table, the URL built by its helper is served back to the
   router and MUST dispatch to that same route. The §V4.6 property is
   not sampled; it is enumerated.

---

# Appendix A — Mapping to Go

*The data layer: generation contract of `volt gen` for schema packages (`nao_models.go`,
`nao_queries.go`, `nao_dyn.go`). Informative in form; in force it is
pinned by the generator goldens, which are gofmt-stable and compiled by
the real Go toolchain on every `go test ./...` run.*

## A.1 Types

Lower-cased declared type, parenthesized arguments ignored. An unmapped
type is a **generation error** naming the column — never a silent guess.

| Declared types | Go |
|---|---|
| `tinyint`, `int2`, `smallint`, `smallserial` | `int16` |
| `int`, `integer`, `int4`, `mediumint`, `serial` | `int32` |
| `bigint`, `int8`, `bigserial` | `int64` |
| `"tinyint unsigned"` / `"smallint unsigned"` / `"int unsigned"` / `"bigint unsigned"` | `uint8` / `uint16` / `uint32` / `uint64` |
| `real`, `float4` | `float32` |
| `float`, `float8`, `double`, `"double precision"` | `float64` |
| `decimal`, `numeric`, `money` | `string` (exact; money never rides a float) |
| `bool`, `boolean` | `bool` |
| `varchar`, `char`, `text` family, `citext`, `string`, `uuid` | `string` |
| `date`, `timestamp*`, `datetime` | `time.Time` |
| `time`, `timetz` (time of day) | `string` (SQLite has no time type; stored as TEXT) |
| `json`, `jsonb` | `json.RawMessage` |
| `bytea`, `blob` family, `binary`, `varbinary` | `[]byte` |
| enum type (optionally schema-qualified) | generated `type X string` + constants |

## A.2 Nullability

A column is required (`T`) when it has `not null`, `pk`/`primary key`
(setting or legacy flag), `increment`, or is covered by a `[pk]` index —
primary keys imply NOT NULL in SQL. Everything else is nullable and
generated as `rt.Null[T]` — a value plus a validity bit, never a pointer
(decisions.md D13). Types where `nil` already expresses NULL (`[]byte`,
`json.RawMessage`) stay bare. In the dynamic layer, nullable columns get
`rt.NullColumn` handles: comparisons take plain `T`, NULL is explicit
(`IsNull`, `SetNull`).

## A.3 Names

`snake_case` → `PascalCase` with the Go initialisms convention
(`user_id` → `UserID`, `api_key` → `APIKey`). Model names are the
singular of the table name (`users` → `User`, D10), overridable with
`[model:]` — the only model-naming override; `resources`' `singular:`
setting (§V5.3) renames the route member helper, never the model.
Non-`public` schemas prefix the type (`core.users` →
`CoreUser`) so equal base names cannot collide. A leading digit is
prefixed (`2fa_codes` → `X2faCode`). Two declarations mapping to the
same Go identifier is a generation error; the `dynname` lint reports
the dynamic layer's concatenation collisions ahead of time.

## A.4 Notes become doc comments

| Note on | Generated position |
|---|---|
| `Project` | package comment |
| `Table` (body form wins over `[note:]`) | struct doc comment |
| column | field doc comment |
| enum value | constant doc comment |
| injected partial column | travels with the column; a §6.9.4 override brings its own note |

Generated files start with the machine-readable
`// Code generated … DO NOT EDIT.` marker; `volt gen` refuses to
overwrite a file lacking it. Generated code imports the standard
library and `nao/rt` only.

## A.5 Struct tags

Every generated struct field derived from a column — on the model and
its params structs alike — carries the same tag, assembled in this
order:

1. `db:"<column name>"` — the scan contract. Not overridable; the
   column setting `tag:` rejects the key `db` (§6.3).
2. `json:"<column name>"` — the JSON default. Nothing is omitted
   (`omitempty` is never generated): a document always carries every
   column, and NULL is the value `null` (D13). A column `tag:` with key
   `json` **replaces** this pair verbatim.
3. Every other `tag:` passthrough pair, verbatim, in declaration order.

A struct derivative (§V11.7) copies each kept field's assembled tag
unchanged; an explicit-list shared row type (§V11.7) carries the
default `db`/`json` pair only.

---

# Appendix B — Mapping to SQLite DDL

*Generation contract of `volt gen --sql` (`nao_schema.sql`). Pinned by
goldens that are executed on a real SQLite (`PRAGMA foreign_keys = ON`,
`foreign_key_check` after), seed INSERTs included.*

| Declared | SQLite |
|---|---|
| enum-typed column | `TEXT` + `CHECK (col IN ('a', 'b'))` — SQLite has no enum type |
| `core.users` (schema) | `core_users` — SQLite has no schemas; collisions are generation errors |
| `pk` column | explicit `NOT NULL` added: SQLite's `PRIMARY KEY` does **not** imply NOT NULL |
| integer `[pk, increment]` | exactly `INTEGER PRIMARY KEY` (rowid alias); `AUTOINCREMENT` deliberately avoided |
| composite pk (`[pk]` index) | `PRIMARY KEY (a, b)` table constraint; members get `NOT NULL` |
| refs (`>`, `<`, `-`) | `FOREIGN KEY … REFERENCES` with `ON DELETE` / `ON UPDATE`; FK side per §6.7.3 |
| one-to-one (`-`) | adds `UNIQUE` on the FK column unless something already guarantees it |
| many-to-many (`<>`) | nothing — model the junction table explicitly |
| ints / bools | `INTEGER` (booleans stored 0/1; `true`/`false` in records and defaults become 1/0) |
| floats | `REAL` |
| decimal / numeric / money | `TEXT` (exact, consistent with Appendix A's `string`) |
| strings / uuid / dates / json | `TEXT` (dates as ISO-8601, SQLite's convention) |
| bytea / blob family | `BLOB` |
| backtick expressions | verbatim; parenthesized in `DEFAULT` as SQLite requires |
| `records` | multi-row `INSERT` statements after all tables, in source order; empty fields become `NULL` |
| unknown type | generation **error** naming the column |

Identifiers are quoted only when needed (not a plain lower-case name,
or a SQLite keyword): `order` → `"order"`, `full name` → `"full name"`,
`2fa_codes` → `"2fa_codes"`. Project note → file header comment; table
and column notes → comments above their DDL. Output opens with
`PRAGMA foreign_keys = ON;` and the `-- Code generated … DO NOT EDIT.`
marker, with the same overwrite protection as Appendix A.

---

# Appendix C — Compatibility with DBML

Volt accepts the entire DBML core of Part I verbatim: a schema using
only Part I constructs is upstream-valid DBML and pastes into
dbdiagram.io unchanged. The compatibility break is **one system**:
DBML's file-based module machinery (Part I §7) — `use * from './file'`,
selective imports with per-element kinds and `as` aliases, and `reuse`
re-export — **is not part of the Volt language** (§V2.5). The parser
still recognizes the §7 forms so tooling can diagnose them precisely;
the checker rejects them with a diagnostic pointing here.

**Why.** All four mechanisms manage one problem — name conflicts caused
by dumping imported declarations into the importer's scope. Volt's
package system (§V1–§V2, qualified access only: `db.users`) makes those
conflicts impossible, so the machinery solving them is deleted rather
than carried. Circular file imports (legal in DBML) become the §V2
cycle error: packages must layer.

**Migration** from a multi-file DBML project, mechanically:

1. Files that imported each other become **one package**: put them in
   one directory, delete the `use`/`reuse` lines, add the same
   `package` clause to each file. Intra-package references need no
   import at all.
2. Genuinely separate concerns become **separate directories**, each
   with its own `package` clause; importers name them in an
   `import (…)` block (§V2) and qualify references (`db.users`).
3. Aliased imports (`use { table users as u }`) have no equivalent and
   need none: the package qualifier disambiguates (`auth.users` vs
   `billing.users`); a package alias (§V2.2) shortens the qualifier.
4. `reuse` re-export chains flatten: each consumer imports the real
   defining package directly.

---

## License and Provenance

Part I is a distillation of the DBML language as documented and
implemented in [holistics/dbml](https://github.com/holistics/dbml)
(the `@dbml/parse` reference implementation). While Part I was being
written, every corpus verdict was cross-checked against that
implementation and the prose corrected wherever it disagreed; the
cross-check harness was retired at zero disagreements (D54), and the
corpus it pinned remains the executable record.
Like the upstream project, this specification is licensed under the
[Apache License 2.0](../LICENSE).
