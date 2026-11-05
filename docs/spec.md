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
  columns, indexes, refs, types, partials, records and doc comments,
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

## Values and effects

The generated code and the runtime keep four rules. They are the
functional style, avoiding mutation and avoiding side effects, held by
the generator and `volt vet` where Go itself holds nothing.

1. **Values.** Rows, changesets, predicates, queries, requests,
   responses, diagnostics and every data structure of the tooling are
   values. A method on a value returns a new value and leaves its
   receiver as it was.
2. **No mutation after construction.** A function may accumulate
   locally, appending to a slice that has not escaped, since nothing
   can observe it. It mutates neither an argument, nor a field of a
   value another holder has, nor package state.
3. **Effects at the edge, as data.** Whatever touches the world is
   described as a value and performed in one place: the generated
   shim, the runner or `main`. Hand-written application code produces
   descriptions.
4. **Mutable state is inventoried.** The connection pool, the
   statement cache, the event broker's log and the logger are the
   whole list.
5. **Pure phases in the tooling.** The scanner, the parser, the
   checker, the fold and the generators are functions from values to
   values. The loader reads a project through a `source.Source` the
   tool or the language server hands it (`lang/source`), every
   generator returns a map from file name to content, and `volt gen`
   writes the maps in one step at the end. No package under `lang/`,
   `gen/` or `nao/gen` imports `os`, `io/fs`, `database/sql` or
   `net`; `cmd/volt/layers_test.go` holds it.

A hand-written handler is a function from a request value to a
response value (§requests-and-responses): it holds no database, reads
the clock from the request, and describes its effects for the
generated shim to perform; the vet rules of `lint.md` hold it to that.
An effect that uses an earlier effect's result carries a function, the
`next` of `rt.Then`, from a value to a description, or the `f` of
`rt.Map`, from a value to a value (§generated-query-surface): that
function is pure too, the runner calls it between the statements,
nothing performs inside it, and it writes no variable it captures.

---

# Part I — The Schema Core

**Status:** Normative.
Part I is Volt's schema language: tables, columns, indexes, refs,
types, partials, records and doc comments, with column types drawn
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

- **Introduction**
  - [Values and effects](#values-and-effects)
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
    - [Table](#table)
    - [Columns](#columns)
    - [Default Values](#default-values)
    - [Indexes](#indexes)
    - [Relationships (Ref)](#relationships-ref)
    - [Types](#types)
    - [TablePartial](#tablepartial)
    - [Records (Sample Data)](#records-sample-data)
    - [Doc Comments](#doc-comments)
  - [Static Semantics](#static-semantics)
    - [Namespaces and Uniqueness](#namespaces-and-uniqueness)
    - [Reference Resolution](#reference-resolution)
    - [Partial Injection Order](#partial-injection-order)
    - [Nullability and Cardinality](#nullability-and-cardinality)
  - [Collected grammar (Part I)](#collected-grammar-part-i)
- **Part II — The Project and Routing Layer**
  - [Layers, files and the superset rule](#layers-files-and-the-superset-rule)
  - [Data and network layers](#data-and-network-layers)
  - [Projects and packages](#projects-and-packages)
  - [Imports](#imports)
  - [Pipelines](#pipelines)
  - [Scopes and routes](#scopes-and-routes)
    - [Route paths](#route-paths)
    - [Routes](#routes)
    - [Handler references](#handler-references)
    - [Requests and responses](#requests-and-responses)
    - [Request bodies](#request-bodies)
    - [Route loads](#route-loads)
    - [Scope loads](#scope-loads)
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
    - [Doc comments in generated code](#doc-comments-in-generated-code)
    - [Struct tags](#struct-tags)
    - [Generated query surface](#generated-query-surface)
  - [Mapping to SQLite DDL](#mapping-to-sqlite-ddl)
  - [License and Provenance](#license-and-provenance)

Sections are named, not numbered. A citation is `§` followed by the
heading's anchor, the heading lowercased with its words joined by
hyphens: `§columns`, `§selects-over-groups`, `§struct-tags`. A
citation never carries a constraint's number; numbers move, names
stay. A semantic error's code is `spec/<anchor>`, a syntax error's
code is `syntax`, and a lint warning's code is `vet/<rule>`
([`lint.md`](lint.md)). A citation that names no heading fails the
build (`lang/spec_doc_test.go`).

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
| `n * x`       | Exactly `n` occurrences of `x` (e.g. `4 * digit`)              |
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
whitespace everywhere else but before a settings list, which stays on
the line of its construct (§settings-lists); a route path and a
package path hold no whitespace or comment between their tokens
(§route-paths, §imports).

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
| `U+0301` | COMBINING ACUTE ACCENT, a combining mark             |
| `U+00E9` | LATIN SMALL LETTER E WITH ACUTE, a precomposed letter |

### Unicode General Categories

Unicode assigns every character a *General Category*. Identifiers are
Go's (§identifiers), and this specification defines them with two
categories, as Go does:

- **Category L (Letter)**: letters of any script, `a` to `z` and `A` to
  `Z`, but also `ż`, `é`, `ß`, `я`, `漢`, `ا`, and the rest. It is the
  union of the subcategories Lu uppercase, Ll lowercase, Lt titlecase,
  Lm modifier and Lo other.
- **Category Nd (Decimal Number)**: decimal digits of any script, `0`
  to `9` among them.

A combining mark (category M, such as U+0301) is neither: the
precomposed `é` (U+00E9) is a letter, while `e` followed by U+0301 is a
letter and a mark, and no identifier. In regular-expression terms the
two categories are `\p{L}` and `\p{Nd}`; an implementation MUST NOT
approximate them with ASCII `[a-zA-Z]` and `[0-9]`.

### Case Sensitivity and Terminology

**Case sensitivity.** Volt is case-sensitive, as Go is. Every *keyword*
(a terminal spelled with letters in this grammar, e.g. `"Table"`,
`"pk"`, `"null"`) has exactly one spelling, the one the grammar gives it
(§keywords), and a name is looked up by its exact spelling: `users` and
`Users` are two names.

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

- `"""` opens a multi-line string, never the empty string `""` followed by `"`.
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
   column definitions and record rows. Productions
   in this specification reference `newline` explicitly wherever it is
   syntactically significant, except before a settings list
   (§settings-lists). A production that ends with
   `newline` is **line-oriented**: it lies on one line, and a line break
   inside it is whitespace only inside brackets it opened (`[ ]`, `( )`,
   `{ }`), so a settings list may span lines and a column's type may not
   sit on the next line. The `newline` that ends it is also satisfied by
   the end of the file and by a following `}`, so a block's last line may
   close the block.
3. Space and tab characters separate tokens and are otherwise
   insignificant, except that they end a route path or a package path
   (§route-paths, §imports).
4. Indentation is significant only inside multi-line strings
   (§multi-line-string-literals) and at element boundaries.
5. **Element boundaries.** An identifier (§identifiers) in the first column
   of a line begins a top-level element (§program-structure) whenever it lies outside every
   string, comment and brace: the brace depth counts `{` up and `}` down,
   never below zero, over the tokens before it. The element before such an
   identifier ends there, complete or not: a parser MUST NOT read past it to
   finish an element, and MAY therefore parse each element from its own
   text. An element's doc comment (§doc-comments) is part of the element:
   when the identifier has one, the element before ends at the first line
   of that doc comment instead. An element that continues onto a further line either indents that
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
   (§table, §imports, §groups, §migrations). What a conforming front end reports
   for invalid input is therefore determined by the input alone; the
   conformance corpus pins every diagnostic of every invalid snippet.

```volt invalid
table users {
  id int [pk]
}

table posts {
  id int [pk]
  author_id int
}

ref: posts.author_id >
users.id                      // error: a new element begins here
```

### Comments

There is **exactly one** comment form, as in Go's doc comments:

```ebnf
comment  = "//", { any char - newline } ;

any char = ? any Unicode character ? ;
```

1. A comment extends to, but does not include, the next `newline` (or
   end of file). It does **not** consume the terminating newline; the
   newline retains its statement-terminating role.
2. `/*` outside a string, an expression literal and a comment is a
   lexical error naming `//`: Volt has no block comment.
3. Comments are trivia: they may appear between any two tokens and have no
   semantic effect, except that one ends a route path or a package path,
   as whitespace does (§route-paths, §imports).
4. A comment that is the only token-free text of its line, after
   spaces and tabs, stands **on its own line**; one that follows a token
   on its line is a **trailing comment**. Consecutive lines that each
   hold a comment on its own line, with no other line between them,
   form a **comment group**. A blank line, or a line holding a token,
   ends the group.
5. A comment group directly above a construct is that construct's doc
   comment (§doc-comments). Every other comment, a trailing one, a
   group a blank line separates from what follows, a group at the end
   of a body, is a comment to the reader and nothing else.

```volt
// A doc comment: the group directly above the table documents it.
table users {
  id int [pk] // a trailing comment documents nothing
}
```

### Identifiers

```ebnf
identifier    = letter, { letter | unicode digit } ;

letter        = ? any character of Unicode category L (Letter) ?
              | "_" ;
unicode digit = ? any character of Unicode category Nd (Decimal Number) ? ;
digit         = "0" | "1" | "2" | "3" | "4"
              | "5" | "6" | "7" | "8" | "9" ;
```

1. An identifier is a Go identifier: a letter or an underscore, then
   letters, digits and underscores, with letters and digits as Go
   defines them (§unicode-general-categories). It is a maximal run of
   these characters.
2. An identifier never begins with a digit. A run of digits is a number
   (§numeric-literals); a run of digits followed by letters is an error
   naming the rule, except as a migration name (§migrations), the one
   token that begins with digits. A combining mark in a name, or before
   one, is an error.
3. A name is written as itself and never quoted: a double quote opens a
   string (§string-literals), and a word that is not an identifier (one
   with a space, a hyphen or a leading digit) is no Volt name.

### Keywords

Volt reserves no word outside a predicate. Every keyword is
contextual: a word acts as a keyword only in keyword position, where
the grammar places it, and is an ordinary identifier everywhere else (a
column may be named `table`, `indexes` or `type`). Inside a predicate
expression the words `and`, `or`, `not`, `in`, `like`, `is`,
`exists` and `null`, in any case, and `true` and `false` where an
operand stands, are reserved: they name no column, no relation and no
`pred` there (§predicates).

Keywords are case-sensitive (§case-sensitivity-and-terminology), and
each has one spelling, all lowercase: the element and block words of
the table below, `package` and `import`, which open elements, the
migration words that open the blocks of a migration body (`up`,
`down`, `create`, `alter`, `rebuild`), and the rest. A word in keyword
position that matches a keyword only when case is ignored is a syntax
error naming the right spelling (`write table, not Table`); the parser
then reads it as that keyword, so the error is reported once.

| Class | Keywords | Section |
|---|---|---|
| element and block words | `table`, `table_partial`, `type`, `ref`, `records`, `group`, `pred`, `select`, `migration`, `pipeline`, `scope`, `indexes`, `validators` | §program-structure, §indexes, §validators |
| package words | `package`, `import` | §projects-and-packages, §imports |
| setting names | `pk`, `null`, `unique`, `index`, `autoincrement`, `default`, `ref`, `tag`, `required`, `model`, `name`, `where`, `delete`, `update`, `from`, `down`, `irreversible`, `no_transaction`, `pipe`, `error_handler`, `api`, `only`, `except`, `param`, `singular`, `strip`, `order`, `group`, `limit`, `as`, `reverse`, `timeout`, `body` | §settings-lists, §settings-whitelists |
| referential actions | `cascade`, `restrict`, `set_null`, `set_default` | §relationships-ref |
| resource actions | `index`, `new`, `create`, `show`, `edit`, `update`, `delete` | §the-action-table |
| order directions | `asc`, `desc` | §selects-over-groups |
| literal words | `true`, `false`, `null` | §boolean-and-null-literals |
| clause words | `from`, `for`, `join`, `with`, `where`, `as`, `and`, `or`, `not`, `in`, `like`, `is`, `exists` | §types, §selects-over-groups, §predicates |
| aggregate words | `count`, `sum`, `avg`, `min`, `max` | §selects-over-groups |
| route words | `get`, `post`, `patch`, `delete`, `options`, `head`, `any`, `resources`, `dataset`, `use`, `load` | §routes, §resources, §datasets, §pipelines, §scope-loads |
| parameter types | `int` | §route-paths |
| migration words | `up`, `down`, `create`, `drop`, `rename`, `alter`, `rebuild`, `exec`, `table`, `index`, `add`, `to` | §migrations |

`records` opens an element at top level and a block in a table body;
`ref` opens an element at top level and names a column setting. A referential
action, a resource action, an order direction and a parameter type are
keywords only as the value of `delete:` or `update:`, in the `only:` or
`except:` list of a `resources` declaration, after a column of
`order:`, and in the parentheses of a path parameter (`:id(int)`)
respectively; an aggregate word is a keyword before `(` in a
projection, and `as` after an item there (§selects-over-groups); a
literal word is a keyword where a value stands. A
bare path parameter is a string, so `int` is the one parameter type
written (§route-paths), and `put` is no route word: PATCH is the partial
update (§routes). The
words of a migration body are keywords inside a `migration` only
(§migrations); elsewhere they are identifiers, so a column may be named
`up` or `to`.

### String Literals

```ebnf
string             = single line string | multi line string ;

single line string = '"', { sls char | escape sequence }, '"' ;
sls char           = any char - ( '"' | "\" | newline ) ;
```

1. A single-line string is delimited by double quotes `"`, as in Go,
   and lies on one line: a line break inside it is an error, and so is
   a backslash before one (§escape-sequences).
2. The escape sequences of §escape-sequences are interpreted; a `"`
   inside is written `\"`, as in `[tag: "json:\"slug\""]`, and a
   single quote stands as itself (`"it's"`).
3. Single quotes delimit nothing: `'...'` and `'''...'''` are errors
   naming the double-quoted spelling. Inside an expression literal a
   single quote is SQL's string delimiter (§expression-literals).

### Multi-line String Literals

```ebnf
multi line string = '"""', mls body, '"""' ;
mls body          = { mls piece } ;
mls piece         = mls plain | escape sequence
                  | '"', ( mls plain | escape sequence )
                  | '""', ( mls plain | escape sequence ) ;
mls plain         = any char - ( "\" | '"' ) ;
```

1. Delimited by triple double quotes `"""`; may span any number of
   lines. Longest match (§tokenization) closes it at the first `"""`,
   so a body holds no three double quotes in a row and does not end
   with one; such a quote is written `\"`, and a closing `"""`
   followed by a further `"` is an error naming that spelling.
2. The escape sequences of §escape-sequences are interpreted, the same
   four as in a single-line string, and a backslash before a line break
   is an error: each line of the value is a line of the source.
3. **Indentation stripping:** after escape processing, a first line
   that holds only spaces and tabs is removed (the common case is an
   empty one, where content starts on the line after the opening
   `"""`), and so is a last line that holds only spaces and tabs,
   with the line break before it (the line of an indented closing
   `"""`). Then compute the minimum number of leading spaces over the
   lines that hold anything else, and remove that many leading spaces
   from every line, or all of its leading spaces from a line that has
   fewer. A tab is not indentation: a line's leading spaces end at its
   first tab.

```volt
table posts {
  id int [pk]
  body string [default: """
    This is a block string.
    It spans multiple lines.
  """]
}
```

The default's value is `This is a block string.\nIt spans multiple lines.`

### Escape Sequences

Escape sequences apply inside single-line and multi-line strings and
nowhere else: an expression literal (§expression-literals) and a
comment take a backslash as itself.

```ebnf
escape sequence = "\", escaped item ;
escaped item    = "\" | '"' | "n" | "t" ;
```

| Sequence | Value                    |
|----------|--------------------------|
| `\\`     | backslash                |
| `\"`     | double quote             |
| `\n`     | line feed (U+000A)       |
| `\t`     | horizontal tab (U+0009)  |

1. These four are the only escapes, a subset of Go's. Every other
   backslash sequence in a string is an error naming the four: `\'`,
   `\r`, `\0`, `\u0041` and `\q` among them. Every other character
   stands in a string as itself.
2. A backslash immediately before a line break is an error wherever it
   stands outside a comment and an expression literal, in a string or
   between tokens: Volt has no line continuation, so a string and an
   element read as they lie in the source.

### Numeric Literals

```ebnf
number   = digit, { digit }, [ ".", digit, { digit } ], [ exponent ] ;
exponent = ( "e" | "E" ), [ "+" | "-" ], digit, { digit } ;
```

1. Examples: `42`, `3.14`, `1e2`, `1.5e10`, `3.14e-5`.
2. A leading sign is not part of the literal; a negative value such as
   `-100` in record rows is parsed as prefix operator `-` applied to a number.
3. At most one decimal point is permitted. A digit run followed by further
   letters (e.g. `2fa`) is no number and no identifier: it stands only
   as a migration name (§migrations, §identifiers). A digit run containing
   a dot followed by letters (e.g. `12.3abc`) is a lexical error.

### Boolean and Null Literals

```ebnf
boolean = "true" | "false" ;
null    = "null" ;
```

Lowercase only, as every keyword outside the element and block words
(§keywords).

### Expression Literals

```ebnf
expression literal = "`", { any char - "`" }, "`" ;
```

1. Delimited by backticks. The content is an opaque, raw SQL expression:
   backslash is **not** an escape character and there is no way to embed a
   literal backtick.
2. May span multiple lines.
3. Used for computed defaults (`` default: `CURRENT_TIMESTAMP` ``), expression
   indexes, partial-index conditions (`where:`, §indexes), the sources
   of a rebuild (`from:`) and the SQL of an `exec` with its `down:`
   (§migrations), and expression values in records. A SQL string
   literal inside one keeps SQL's single quotes
   (`` where: `country != ''` ``).

### Operators and Punctuation

```ebnf
punct = "{" | "}" | "[" | "]" | "(" | ")"
      | "," | ":" | "." | "~" | "*" | "/"
      | "<" | ">" | "-" | "+" | "\" | "="
      | "!=" | "<=" | ">=" | "?" ;
```

These are every token that is neither a name nor a literal; any other
character outside a string, an expression literal, an identifier or a
comment, `;`, `#` and `'` among them, is a lexical error. Their roles:

| Token   | Role                                                        |
|---------|-------------------------------------------------------------|
| `{ }`   | element bodies and the blocks inside them                   |
| `[ ]`   | settings lists, and the `[]` of the type `[]byte`           |
| `( )`   | composite column lists (index keys, `table.(a, b)` endpoints), records column lists, group sets (§groups), select projections and their exclusion sets (§selects-over-groups), import blocks (§imports), a path parameter's type (§route-paths), validator arguments (§validators), the lists of `only:`, `except:`, `group:` and `order:` (§settings-whitelists), predicate grouping and `in` lists (§predicates) |
| `:`     | a setting value, the body of `ref:`, a route parameter (§route-paths), a query parameter (§predicates), also as a select's `limit:` (§selects-over-groups) |
| `,`     | separator inside `[ ]`, `( )` and record rows               |
| `.`     | qualification: `table.column`, `pkg.Name` (§types), `pkg.name` (§imports), `Controller.Action` (§handler-references); three contiguous `.` end a wildcard, `:name...` (§route-paths) |
| `~`     | `table_partial` injection prefix                               |
| `>`     | a relationship, foreign key first (§relationships-ref); a comparison in a predicate (§predicates) |
| `-`     | the sign of a negative number                               |
| `*`     | every column of a select's target (§selects-over-groups)    |
| `/`     | route paths (§route-paths) and import paths (§imports)      |
| `+ \`   | group union and difference (§groups); `\` also leaves columns out of a star projection, `(* \ a)` (§selects-over-groups) |
| `= != < <= >=` | comparisons in a predicate (§predicates); `=` also defines a group by expression, `Group g = a + b` (§groups) |
| `?`     | an optional query parameter, written directly after its name, `:name?` (§predicates) |

---

## Common Syntactic Forms

### Names and Qualified Names

```ebnf
name = identifier ;
```

1. A table, a partial and a column are each named by one `name`, and a
   type of the package by one identifier (§types); no name is
   qualified by a schema or any other container.
2. The qualifications Volt has are `table.column` and `table.(a, b)`
   in a relationship (§relationships-ref), `pkg.Name` naming a Go type
   (§types), `Controller.Action` naming a handler method of the routes
   package (§handler-references), and `pkg.name` naming a declaration
   of an imported package (§imports) or, as `volt.Name`, of the
   runtime. A package names its own declarations
   bare: a dotted reference to a declaration always names an import
   (§imports). A dotted name is neither a default value
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
   pair (`model: "Account"`, `default: 123`).
2. Setting names are lowercase keywords, matched exactly (§keywords). A
   setting name is one
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

element = table
        | table partial
        | type decl
        | ref element
        | records element ;
```

1. A program is a sequence of top-level elements in any order, except
   that a file's package clause comes first (§projects-and-packages);
   Part II extends `element` (§collected-grammar-part-ii). An element
   begins where the one before it is complete: after its closing `}`,
   on the same line or a later one, or on a later line after a
   line-oriented element. An identifier in the first column outside
   every brace, string and comment always begins one, ending the
   element before it (§line-structure-and-whitespace).
2. Forward references are permitted: an element may reference another
   element defined later in the file, or in another file of its package
   (§projects-and-packages). The language is fully declarative:
   declaration order carries no meaning, and fixes only the order of
   generated output and of diagnostics (§projects-and-packages).

---

## Element Definitions

### Table

```ebnf
table          = "table", name, [ table settings ],
                 "{", table body, "}" ;

table body     = { column
                 | indexes block
                 | validators block
                 | partial injection
                 | records block
                 } ;

table settings = "[", table setting, { ",", table setting }, "]" ;
table setting  = "model", ":", string ;
```

1. A table MUST contain at least one column, either directly or via an
   injected `table_partial` (§tablepartial).
2. **Name.** A table has one name, a single `name`: there is no alias
   and no schema qualifier. That name is the table's SQLite name,
   quoted only when needed (§mapping-to-sqlite-ddl). Tables and groups
   share one namespace in a package (§groups).
3. **Settings.** `model` is the only table setting. `model:` pins the
   Go model name used by `volt gen` (by default the singularized table
   name, §names); the `vet` rule [`modelname`](lint.md#modelname) tells
   you when it is needed. A table is documented by its doc comment
   (§doc-comments).
4. Column names MUST be unique within a table after partial injection (§partial-injection-order).

```volt
// Registered accounts
table users {
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
                | "index"
                | "autoincrement"
                | "required"
                | "default", ":", default value
                | "tag", ":", string
                | "ref", ":", name, ".", name ;   (* table.column, §relationships-ref *)
```

1. **Type.** A column type is a Go type, matched case-sensitively, from
   a closed set: the built-ins `bool`, `int`, `float64`, `string`,
   `[]byte` and `time.Time`, or a type declared with `type` (§types). Anything else is an error naming the column and
   the set; the `[]` of the grammar belongs to `[]byte` alone, so
   `[]string` is an unknown type. An unknown type is reported once, at
   the column, even when a `table_partial` injects the column into several
   tables (§tablepartial): a select, a validator or a route that uses
   the column reports nothing further about its type. A select that
   uses such a column, in its predicate, its order, its projection or
   as a column its row type keeps, is kept unresolved: it generates no
   method, and the query routes and datasets that name it are checked
   as for any select (§query-routes, §dataset-declaration). A query
   route to a default CRUD function keyed by such a column, and a
   resources declaration over a table whose primary key is one, are
   kept and checked the same way (§query-routes, §table-resolution).
   One check is narrowed: a path parameter that a dropped predicate
   term over the column names, and none of the select's remaining
   parameters does, is not reported, since it belongs to the dropped
   term. An error that does not depend on the type, such as a group
   member that lacks the column, an `order:` term without a direction,
   a route conflict, a setting a route, resources or dataset does not
   take, or a validator function that does not exist, is reported as
   usual. A column whose type names a `type` declaration in error
   reports nothing: the declaration's error (§types) stands for it.
   The type decides the Go field
   (§mapping-to-go), the SQLite column (§mapping-to-sqlite-ddl), the
   literals a `default:` accepts (§default-values), the empty value
   `required` refuses, and the comparisons a predicate allows
   (§predicates).
2. **Nullability.** A column is NOT NULL unless it carries `null`,
   which admits NULL. This mirrors Go, where a value is never nil
   unless its type says so, and it holds for every type alike: a column
   that carries `null` is `rt.Null[T]`, `[]byte` among them, and a
   built-in type binds NULL only as an invalid `rt.Null`
   (§nullability). A `[]byte` field is therefore bytes and never NULL:
   nil and a non-nil empty slice are one value, empty bytes, which
   every bind of a `[]byte` writes as `X''`, in a changeset write, a
   key, the dynamic layer and a valid `rt.Null[[]byte]` alike, and
   which every read hands back as a non-nil empty slice whatever the
   driver returns (§nullability). The zero value of a `[]byte` column
   without `null` thus satisfies NOT NULL, as every other built-in
   type's does, and `required` is the one refusal of empty bytes, as it
   is of `""` for `string`. A declared type (§types) stored as `[]byte`
   is not a plain `[]byte`: its own `driver.Valuer` decides what it
   binds, and a value of any declared type whose `driver.Valuer`
   returns nil binds as NULL. A key column, one carrying `pk`, MUST
   NOT carry `null`.
3. **`pk`** marks the column a member of the table's primary key, and
   is the one spelling of a primary key: an index takes no `pk`
   (§indexes). The primary key is the columns that carry `pk`, in
   the table's effective column order (§partial-injection-order):
   one such column is a single-column key, several form one composite
   key. The key's column order is the column list of its `PRIMARY KEY`
   (§mapping-to-sqlite-ddl) and the parameter order of `<Model>Get`,
   `<Model>Delete` and the key `<Model>Update` writes by
   (§generated-query-surface). A primary key is a table constraint, not
   an index: no migration adds it to a table or drops it from one, and
   changing it is a `rebuild table` (§migrations).
4. **The rowid alias, the database-assigned key and `autoincrement`.**
   The **rowid alias** is the DDL fact: a table whose primary key is
   exactly one column stored as `int`, of the built-in `int` or of a
   declared type (§types), makes that column SQLite's rowid alias,
   `INTEGER PRIMARY KEY` without `NOT NULL` (§mapping-to-sqlite-ddl).
   An insert that marks it stores the value given, one that leaves it
   unmarked gets a rowid SQLite picks, and SQLite ignores a `DEFAULT`
   on it, so `default:` on the rowid alias is an **error**
   (§default-values): Volt accepts no setting the database would not
   apply. The **database-assigned key** is the rowid alias of the
   built-in `int` that is no foreign key, neither carrying `ref:` nor
   a column of the left endpoint of a `ref` element
   (§relationships-ref): the database assigns it whenever an insert
   leaves it unmarked (§changesets), and `id int [pk]` is the key most
   tables want. Every other rowid alias is an **ordinary key**, whose
   value the row gives: a key of a declared type stored as `int`, and
   a key that is also a foreign key, as in the shared-key one-to-one
   `user_id int [pk, ref: users.id]`, whose value comes from the
   referenced row. `From` marks an ordinary key, a generated create
   casts it, a records list names it, `required` may be written on it,
   and an insert that leaves it unmarked is refused as `not_null`
   rather than given a rowid SQLite picks (§changesets). `required` on the database-assigned key is an **error**, since
   the database may assign it, so its zero value is no missing value
   (rule 8). A records list may leave the database-assigned key out
   (§records-sample-data).

   **`autoincrement`** is SQLite's `AUTOINCREMENT` and does what
   SQLite says it does: the database-assigned key becomes `INTEGER
   PRIMARY KEY AUTOINCREMENT`, so the database never reuses an id, not
   even the largest one after its row is deleted, and SQLite keeps the
   high-water mark in its `sqlite_sequence` table, which a `rebuild
   table` that keeps `autoincrement` carries over (§migrations). It
   does nothing else: the key is the database-assigned key with or
   without it. It is valid only on the database-assigned key and is
   an **error** anywhere else: on a column that is not of type `int`,
   on a column that is not the primary key and on a member of a
   composite key, where SQLite refuses it; on a declared type stored
   as `int`, which SQLite would accept but Volt refuses, since it
   treats that key as an ordinary key (above); and on a rowid alias
   that is a foreign key, whose values come from another table, so
   the never-reuse bookkeeping has nothing to assign.
5. **`ref: table.column`** declares a relationship whose foreign key
   is this column (§relationships-ref). It appears at most once on a
   column (§settings-lists) and carries no referential actions; a
   relationship that needs `delete:` or `update:` is a `ref` element.
6. **`unique` and `index`** give the column an index of its own, keyed
   by the column alone: `unique` a unique index, `index` a plain one.
   They are the one spelling of a single-column index (§indexes): the
   DDL writes a `CREATE UNIQUE INDEX` or `CREATE INDEX` named
   `idx_<table>_<column>` after the table, never an inline `UNIQUE`
   (§mapping-to-sqlite-ddl). A column carries at most one of the two,
   since a unique index already indexes the column: both together are
   an error. A `unique` column is unique over the whole table, not
   only on the rows a condition admits, so it gets
   `<Model>GetBy<Column>` unless it is the whole primary key
   (§generated-query-surface), and may be the target of a foreign key
   (§relationships-ref).
7. Each column definition is terminated by a line break; the settings
   list is the only thing that follows the type.
8. **`required`** declares that a value must be present in the
   zero-value sense: a zero value that is set, or decoded into a row
   and passed to `From` (§changesets), satisfies a NOT NULL column, nil
   for `[]byte` among them since it binds as empty bytes (rule 2),
   unless a declared type's `driver.Valuer` binds it as NULL, so
   `required` is the word for "and not that either". It is the one
   built-in rule of `Validate` (§changesets), which the schema switches
   on per column, the field error with check `required`: it refuses
   `""` for `string`, `0` for `int` and `float64`, a zero length for
   `[]byte` (nil or empty, and no other value), and for a declared type
   the empty value of its storage type. It is an error on a `null`
   column, which has no empty value to refuse since NULL is absence, on
   a `default:` column, on the database-assigned key (rule 4), and on
   `bool` and `time.Time`, where every value is a value.
9. **`tag: string`** (repeatable) passes one Go struct tag through to
   the generated field verbatim: every field property lives in the
   schema, and the passthrough serves every encoding and third-party
   library without minting a setting name per format.
   1. The value MUST be exactly one `key:"value"` pair in Go struct-tag
      form: a key containing no spaces, colons, double quotes or
      backticks, then a colon, then a double-quoted value containing no
      double quotes or backticks. The pair is a Volt string, so its
      double quotes are written `\"` (`[tag: "json:\"slug\""]`).
   2. Within one column, tag keys MUST be unique across all `tag`
      settings.
   3. The key `db` is reserved and an error, since the `db` tag is the
      scan contract (§struct-tags).
   4. A `json` tag replaces the generated default; every other tag is
      appended after the generated pair, in declaration order
      (§struct-tags).

   `encoding/gob` ignores struct tags entirely: renaming a field for
   gob is a Go-field-name concern, not a tag concern, and has no
   surface here.

```volt
table users {
  id        int [pk]
  user_name string [tag: "json:\"userName\"", tag: "xml:\"user-name,attr\""]
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
| string        | `default: "direct"`                                   |
| boolean       | `default: false`                                      |
| expression    | `` default: `datetime('now', '-5 days')` ``           |

A bare identifier other than `true` and `false`, and a dotted name
such as `state.pending`, is **not** a valid default value.

1. **Typed.** A default MUST fit the column's type (§columns): an
   integer literal for `int`; a number for `float64`; a string for `string`;
   `true` or `false` for `bool`; for a declared type (§types), a
   literal of its storage type, so the default of a value set is the
   value as written (`default: "pending"`). An expression literal is accepted on any column and is
   SQLite's to judge, except on a `time.Time` column. `[]byte` and
   `time.Time` columns take only an expression literal. On a
   `time.Time` column, a declared type stored as one too, the
   expression MUST be one SQLite evaluates to the stored form of a
   `time.Time` (§go-types), so that the row read back binds as the
   text it holds; parentheses around all of it change nothing, and
   it is one of:
   - `CURRENT_TIMESTAMP` or `NULL`, in any case;
   - one `datetime(...)` call none of whose string arguments is the
     modifier `'subsec'`, which writes three fractional digits, or
     `'localtime'` or `'utc'`, which shift the instant by the
     server's zone (`` `datetime('now', '+1 day')` ``);
   - one SQL string (`` default: `'2024-01-15 10:30:00'` ``) that is
     the stored form, as a record value must be
     (§records-sample-data); other date/time text in it is an error
     spelling the stored form of the same instant.
   Any other expression, such as `strftime(...)`, `julianday()`,
   `unixepoch()`, `date()` or a concatenation, is an error naming the
   accepted forms: SQLite writes text in another form, a number Volt
   does not compare as an instant, or a value Volt cannot read. A
   sub-second instant is set from Go by the write itself, whose
   binding trims the fraction (§go-types). A default that does not fit is an
   error naming the column and its type.
2. **NULL has one spelling.** `null` is not a default value: a column
   that carries `null` and has no `default:` is NULL when a write
   leaves it out, so `default: null` would only say that twice and is
   an error.
3. **Not on the rowid alias.** A default on the rowid alias (§columns), the database-assigned key with or without `autoincrement`,
   a key that is also a foreign key and a declared type stored as `int`
   alike, is an error citing §columns: SQLite ignores a `DEFAULT` on
   the rowid alias and assigns a rowid to an insert that leaves the
   column out, so the default would never be written. The same holds
   in a migration's `create table` and `rebuild table` shapes
   (§migrations).

### Indexes

An index is declared one of two ways. A single-column index is the
column setting `unique` or `index` (§columns); every other index, a
composite one, one over an expression, or a partial one, is an entry
of an `indexes` block, which may appear inside a `table` or
`table_partial` body. A body MAY contain more than one `indexes` block;
their contents accumulate.

```ebnf
indexes block  = "indexes", "{", { index }, "}" ;
index          = index key, [ index settings ], newline ;

index key      = index atom
               | "(", index atom, { ",", index atom }, ")" ;
index atom     = name | expression literal ;

index settings = "[", index setting, { ",", index setting }, "]" ;
index setting  = "name", ":", string
               | "unique"
               | "where", ":", expression literal ;
```

1. An index key is a single column, a single expression, or a parenthesized
   composite of columns and/or expressions.
2. Every `name` in an index key MUST refer to a column of the enclosing
   table (after partial injection).
3. **What a block holds.** An entry of an `indexes` block is composite,
   has an expression atom, or carries `where:`. A plain single-column
   index, one column without `where:` and without an expression,
   parenthesized or not, is an error that names the column setting,
   `unique` or `index`, which is its one spelling (§columns).
4. **`unique`** makes the index unique. A unique index with an
   expression atom or `where:` constrains what its expression or its
   condition admits, not its columns: it makes no column unique, so it
   gives no `<Model>GetBy<Column>` (§generated-query-surface) and is no
   target of a foreign key (§relationships-ref). A composite unique
   index without either makes its columns unique together.
5. **No `pk`.** `pk` is a column setting only (§columns): an index
   carrying `pk` is an error naming the column form. A primary key is
   a table constraint and no index; SQLite enforces a unique index and a
   `UNIQUE` constraint the same way, but an index can be added to a
   table that has rows and dropped from it without a rebuild, while a
   primary key cannot.
6. **Name.** An index without `name:` whose key is columns only is
   named `idx_<table>_<col>[_<col>...]` from the SQLite table name
   and its key columns in order (§mapping-to-sqlite-ddl), so that a
   schema and a migration (§migrations) name one index one way; the
   index of a column's `unique` or `index` setting is
   `idx_<table>_<column>`. `name:` overrides the derived name; a
   written name is not empty and does not begin with `sqlite_`, which
   SQLite keeps for its own objects. An index with an expression atom
   MUST carry `name:`, since no name derives from an expression.
   SQLite names an index once in the database, and in the namespace of
   its tables: two indexes of the schema, its tables' columns' indexes
   among them and those partials inject, MUST NOT have one name,
   derived or written, and an index MUST NOT take the name of a table
   of the package (§namespaces-and-uniqueness). Names compare as SQLite
   compares them, the case of ASCII letters aside. The rule holds
   across tables as within one, since a derived name joins names with
   underscores and two tables can derive one (`a_b` with column `c`
   and `a` with column `b_c`), and within one partial, whose names
   derive for every table that injects it (§tablepartial). A
   partial's `name:` is the same in every table that injects the
   partial, so a partial whose index carries `name:` is injected into
   at most one table.
7. **`where`** takes an expression literal and makes the index
   partial: only the rows for which the condition holds are indexed,
   so the index stays small, and a unique partial index constrains
   only those rows. The DDL carries it as `WHERE <expr>`, verbatim and
   SQLite's to judge.
8. An index carries no method: SQLite has one index method. Its doc
   comment (§doc-comments) says why it exists.

```volt
table bookings {
  id           int       [pk]
  country      string    [pk]
  booking_date time.Time [index]
  reference    string    [unique]
  cancelled_at time.Time [null]

  indexes {
    (country, booking_date) [name: "idx_country_date"]
    (country, `lower(reference)`) [unique, name: "idx_bookings_reference_ci"]
    booking_date [name: "idx_bookings_live", where: `cancelled_at IS NULL`]
  }
}
```

### Relationships (Ref)

A relationship declares a foreign key. It has one direction and two
spellings: the `ref` element, and the column setting `ref:` (§columns).
Both write the foreign key first.

```ebnf
ref element  = "ref", ":", ref endpoint, ">", ref endpoint,
               [ ref settings ] ;

ref endpoint = name, ".", column group ;
column group = name
             | "(", name, { ",", name }, ")" ;

ref settings = "[", ref setting, { ",", ref setting }, "]" ;
ref setting  = "delete", ":", ref action
             | "update", ":", ref action
             | "as", ":", name
             | "reverse", ":", name ;
ref action   = "cascade" | "restrict" | "set_null" | "set_default" ;
```

1. **Direction.** The left endpoint is the foreign key and the right
   endpoint the columns it references: `Ref: posts.user_id > users.id`
   reads "posts.user_id references users.id". `>` is the only
   relationship operator, and a `ref` has no name and no braced form.
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
   `ref` and the declaring column of a `ref:`, without exception: the
   checker, the lint rules, the DDL (§mapping-to-sqlite-ddl), the
   generated Go and the shapes of a migration (§migrations) all read
   it so.
5. **One-to-one.** There is no one-to-one operator. A one-to-one
   relationship is a foreign key that is also unique: `unique` on the
   foreign-key column, the foreign-key columns being the table's
   primary key, or a unique index without `where:` over exactly the
   foreign-key columns, beside the reference, as in
   `user_id int [unique, ref: users.id]`. The uniqueness is the
   column's, and the DDL and the generated query surface treat it as
   any unique column's (§mapping-to-sqlite-ddl, §generated-query-surface).
   A **shared-key** one-to-one makes the foreign key the table's whole
   primary key, as in `user_id int [pk, ref: users.id]` or a `ref`
   element whose left endpoint is that key: of the built-in `int`, it
   is the rowid alias in the DDL but no database-assigned key
   (§columns), since its value is the referenced row's, so `From`
   marks it, the generated create casts it, an insert that leaves it
   unmarked is refused as `not_null` (§changesets), a records list
   names it and `autoincrement` on it is an error.
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
8. Whether a foreign key may reference no row is not written
   explicitly; it follows from the nullability of its columns
   (§columns, §nullability-and-cardinality).
9. Two relationships that pair the same columns are invalid, whichever
   side is the foreign key and in whatever order a composite lists its
   pairs: `a.x > b.y` and `b.y > a.x` relate the same columns, and so
   do `a.(x, y) > b.(p, q)` and `a.(y, x) > b.(q, p)`. A composite that
   pairs the same column sets differently, as `a.(x, y) > b.(q, p)`
   does, is another relationship.
10. A `ref` element declares exactly **one** relationship; to declare
    several, write several `ref` elements.
11. **Targets.** The columns a foreign key references are unique in
    their table: exactly its primary key's columns, one column that
    carries `unique`, or exactly the key of a unique index whose key
    is columns only and which has no `where:` (§columns, §indexes); a
    unique index enforces it as a `UNIQUE` constraint would, so SQLite
    takes it as the parent key. A partial or an expression unique index
    makes no column unique and is no target, and neither is one member
    of a composite primary key. SQLite answers any other parent key
    with a foreign key mismatch error at the first write that checks
    the key; the checker does not refuse it, and the vet rule
    [`reftarget`](lint.md#reftarget) reports it.
12. **Relation names.** Every relationship whose endpoints resolve
    gives each of its tables a **relation**, the name a select joins
    by (§selects-over-groups). The foreign-key table's relation, the
    **forward** one, leads to the one row the key references and is
    named after the foreign-key column: the column name without a
    trailing `_id` (`metric_id` names `metric`), else the column name,
    and the referenced table's name for a composite key. The
    referenced table's relation, the **reverse** one, leads to the
    rows that reference it and is named after the foreign-key table
    (`points`). A forward relation is **optional** when any of its
    columns carries `null`; a reverse relation always is, since no row
    need reference a given one. The name maps to a Go name (§names),
    which prefixes the fields of the columns reached through it.
13. **Ambiguity, `as:` and `reverse:`.** A table's relations and its
    own name form one namespace, which the derivation does not keep:
    two relationships from one table to another repeat the reverse
    name, two composite ones the forward name too, and the reverse
    relation of a self-reference takes the table's own name. A name
    two relations share, or that is the table's, read as SQLite reads
    an alias, without regard to case, is **ambiguous**:
    nothing can join it (§selects-over-groups), and the error names
    the relationships and the fix. A name that one relation alone
    carries but for case is no ambiguity and no relation, Volt's names
    being case-sensitive (§case-sensitivity-and-terminology): the error
    names the relation it misspells. The settings of a `ref` element
    name its relations outright, `as:` the forward one and `reverse:`
    the reverse one, each an identifier that maps to a Go name; they
    exist for that case alone. An inline `ref:` carries no settings
    (rule 2): to name its relations, write it as a `ref`.

```volt
table users {
  id int [pk]
}

table posts {
  id int [pk]
  user_id int [null, ref: users.id]
  editor_id int [null]
}

table profiles {
  user_id int [unique, ref: users.id]
}

table merchants {
  id           int    [pk]
  country_code string [pk]
}

table merchant_periods {
  merchant_id int
  country_code string
}

ref: posts.editor_id > users.id [delete: set_null, update: cascade]

ref: merchant_periods.(merchant_id, country_code) > merchants.(id, country_code)
```

### Types

A **Type** declaration admits a Go type of the application as a column
type. The schema names the Go type, where it comes from and how SQLite
stores it; the Go type does the rest.

```ebnf
type decl    = "type", type name, storage type, [ "from", string ], newline ;
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
   is an error, as is `pkg.Name` without `from`. One alias names one
   package: two Type declarations of a package that write one `pkg`
   with two `from` paths are an error naming both, since generated code
   imports each path under its alias.
   Declared names are unique within the package and MUST NOT spell a
   built-in type. A declared type names an exported Go type: `Name`,
   bare or after `pkg.`, MUST begin with an upper-case letter, since
   models other packages import carry it and no other package can
   name an unexported type; the import names and the unexported
   helpers of generated code are thereby never taken. A bare name
   shares the package scope with the exported names generated code
   declares (§names), so it MUST NOT be one of them: a model, a
   changeset with its aliases and constructor, a column handle, an
   option wrapper, a query function (§generated-query-surface), in a
   package that declares migrations `Migrations` (§migration-tooling), and in a package that routes the router's
   `Controllers`, `NewRouter`, `table`, each `<Name>Controller`
   interface and each `Path<Helper>` function
   (§route-names-and-reverse-urls), and `Client` and `NewClient`, which
   the client declares when `-o` writes it beside the models
   (§generated-client); the error on a model's name
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
   by kind, and a type stored as `[]byte` is never a plain `[]byte`
   (§columns): its `driver.Valuer` and
   `sql.Scanner` are the authority on what it binds and reads, nil and
   NULL included, and nothing generated code or the runtime does to a
   plain `[]byte` touches it. The declaration is read by its spelling, as a
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
   named `Type pkg.Name storage from "path"`. `-o` naming the
   package's own directory is not this error: models, queries and
   router land where `gen` without it writes them, and the client
   lands beside the models as `volt_client.go` (§generated-client).

```volt
type money.Money int    from "example.com/app/money"
type img.Png     []byte from "example.com/app/img"
type Slug        string

table products {
  id    int         [pk]
  slug  Slug        [unique]
  price money.Money
  icon  img.Png     [null]
}
```

A value set (rule 5): the schema names the type and the validators,
and the Go file beside it declares the type, its constants and the
validators.

```volt
type OrderState string

table orders {
  id     int        [pk]
  status OrderState [default: "pending"]
  prev   OrderState [null]

  validators {
    OrderStateKnown(status)
    OrderStateKnownNull(prev)
  }

  records (id, status) {
    1, "paid"
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

A `table_partial` declares a reusable fragment of a table body. Tables inject
partials by name.

```ebnf
table partial     = "table_partial", name, "{", partial body, "}" ;
partial body      = { column
                    | indexes block
                    | validators block
                    } ;

partial injection = "~", name, newline ;
```

1. Partial names live in their own namespace and MUST be unique.
2. A partial body is a table body without `records` and without nested
   partial injections. A partial takes no settings: a table's `model`
   is its own.
3. An injection `~p` inside a table body replaces itself with the entire
   content of partial `p` at that position. Injection order is source
   order.
4. **Conflict resolution** when the same column is defined more than
   once:
   1. A column written directly in the table overrides any partial's.
   2. Otherwise the **last-injected** partial (in source order) wins.
   3. The winning definition takes the position of the column's first
      definition (§partial-injection-order).
5. **`indexes` are not overridden.** An `indexes` entry a partial
   injects whose key is the key of an `indexes` entry of the table, or
   of one another injection brings (another partial, or the same
   partial injected again), is an error: an index is declared once, in
   the table or in one partial. Two indexes on one key inside one
   partial are not this error; they are that partial's own redundancy,
   as two in a table body are, and two of one name, derived or
   written, are the error of §indexes. The index of a column's
   `unique` or `index` setting comes with the column and is compared
   with no `indexes` entry; only the name rule of §indexes holds it. A
   partial injected twice lists each of its validators once.

```volt
table_partial base_template {
  id int [pk]
  created_at time.Time [default: `CURRENT_TIMESTAMP`]
}

table users {
  ~base_template
  name string
}
```

### Records (Sample Data)

`records` declares sample rows for a table, in CSV-like syntax. `records` may
be declared at top level (naming the table) or inside a table body.

```ebnf
records element = "records", name, records columns,
                  "{", { record row }, "}" ;

records block   = "records", [ records columns ],
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
   `time.Time`; `true` or
   `false` for `bool`; for a declared type, a literal of its storage
   type (`"paid"` for a value set stored as `string`, §types). `null`
   is NULL and is allowed only on a column that carries `null`.
   A `[]byte` column takes an expression literal or NULL. An expression
   literal (backticks) is accepted on any column and is SQLite's to
   judge.
   A `time.Time` value, of a declared type stored as one too, MUST be
   the stored form of a `time.Time` (§go-types), SQLite's datetime
   text in UTC (`"2024-01-15 10:30:00"`, `"2024-01-15 10:30:00.5"`),
   so that the row read back binds as the text it holds. Other
   date/time text, with a `T`, a `Z` or an offset, without seconds, a
   date alone or a fraction with trailing zeros, is an error whose
   message spells the stored form of the same instant; the checker
   refuses rather than rewrites it, so the source shows the text the
   database holds. An expression literal on such a column is held to
   the expressions a `time.Time` default is (§default-values):
   `CURRENT_TIMESTAMP`, `NULL`, a `datetime(...)` call without the
   `'subsec'`, `'localtime'` or `'utc'` modifier, or a SQL string
   (`` `'2024-01-15 10:30:00'` ``) of the same form, parentheses or
   not; any other expression is an error. A bare identifier other than `true`, `false` and `null`, and
   a dotted name, is **not** a value. A value that does not fit is an
   error naming the column and its type.
5. **No empty field.** Every field of a row holds a value: NULL is
   written `null`, and nothing between two commas, before the first or
   after the last is an error.
6. **Coverage.** A records column list MUST name every column of the
   table, injected columns included (§partial-injection-order), that
   is NOT NULL, has no `default:` and is not the database-assigned key
   (`int [pk]`, with or without `autoincrement`, that is no foreign
   key, §columns), since only the database-assigned key leaves a
   column's value to the database, so every other such column is the
   row's to give, a shared key that is also a foreign key among them,
   whose value is the referenced row's. An omitted list names every
   column (rule 1).

```volt
type JobState string

table users {
  id int [pk]
  name string [null]
  status JobState [null]

  records (id, name, status) {
    1, "Alice", "created"
    2, "Bob", "running"
    3, null, null
  }
}

table posts {
  id int [pk]
  user_id int [ref: users.id]
  posted_at time.Time
}

records posts(id, user_id, posted_at) {
  1, 1, `CURRENT_TIMESTAMP`
  2, 2, "2024-01-15 10:30:00"
}
```

### Doc Comments

A doc comment documents the construct directly below it, as a Go doc
comment does. It is the one way to document anything in Volt.

1. **Attachment.** A comment group (§comments) documents a construct
   when the group's last line is the line directly above the
   construct's first token and that token is the first on its line.
   The rule is the same for every construct that begins a line: an
   element (§program-structure), the package clause included, and an
   item of any body, such as a column, an `indexes` or `validators`
   block and each of its entries, a partial injection, a `records`
   block and each of its rows, an import, a plug, a scope, a route, a
   load, a resources or dataset declaration, a group member, and a
   migration's operations and their items.
2. **Separation.** A group that a blank line separates from the line
   below documents nothing, nor does a trailing comment, nor a group
   above a closing `}`. Only the group directly above counts: a
   comment to the reader sits apart from the construct, with a blank
   line below it, or trails a line.
3. **Text.** The doc text is the group's lines with `//` removed, and
   one space after it when there is one. Trailing spaces and tabs of
   each line, and the empty lines at the start and end of the group,
   are removed, and a run of empty lines becomes one, a paragraph
   break. A comment takes every character as itself: there are no
   escapes (§escape-sequences). The text is conventionally Markdown,
   in Go's doc comment syntax.
4. **Meaning.** A doc comment never changes what a program means: no
   generated statement or identifier depends on one, and the one check
   it meets is that of the routes sharing an action, at most one
   carries a doc comment (§handler-references). Generated Go carries
   the doc comment where its construct becomes a Go declaration
   (§doc-comments-in-generated-code); every other one is the reader's,
   shown by the language server ([`editor.md`](editor.md)).
5. **Package.** The doc comment of the package clause documents the
   package. Each `.volt` file of a package may write one; the
   package's doc text is theirs joined in file name order, each a
   paragraph. A comment at the top of a file with a blank line below
   it, a license or a file header, documents nothing.
6. **Partials.** A column a partial injects keeps its doc comment in
   every table that injects it; a column of the table that overrides
   it (§tablepartial) carries its own, or none.

```volt
// The blog: posts and the people who write them.
package db

// Kept apart by the blank line below, this comment documents nothing.

// A person who writes posts.
table users {
  id int [pk]
}

// A blog post.
//
// Its author sees it before anyone else.
table posts {
  id int [pk]
  // Shown in lists
  title string // a trailing comment documents nothing
  author_id int

  indexes {
    // Lists a writer's posts in order.
    (author_id, id)
  }
}

// Who wrote the post.
ref: posts.author_id > users.id
```

---

## Static Semantics

### Namespaces and Uniqueness

Within one package (§projects-and-packages):

1. Table names MUST be unique. Tables and groups share this namespace
   (§groups): a group MUST NOT take a table's name. Since a table name
   is its SQLite name (§table), it MUST NOT begin with `sqlite_`, in
   any case, which SQLite keeps for its own objects.
2. Declared Type names (§types) form their own namespace and MUST be
   unique. A bare one also shares the Go package scope with the names
   generated code declares and MUST NOT take one of them (§types).
3. `table_partial` names form their own namespace and MUST be unique.
4. Column names MUST be unique within their table.
5. Index names, derived or written, MUST be unique in the package's
   schema and MUST NOT take a table's name, compared without the case
   of ASCII letters: SQLite names tables and indexes in one namespace
   per database (§indexes).

### Reference Resolution

1. Relationship endpoints, index columns and records columns MUST
   resolve to existing elements.
2. Composite relationship endpoints MUST have equal arity, and referenced
   column lists must match in count and order.

### Partial Injection Order

1. Injections are expanded in source order; the effective column order of a
   table is the concatenation of injected and direct columns in source
   order (this order also drives implicit records column lists, §records-sample-data).
   A column defined more than once stands at the position of its first
   definition, whichever definition wins (§tablepartial).
2. Conflicts resolve per §tablepartial.

### Nullability and Cardinality

1. A column that carries `null` is nullable; every other column is
   NOT NULL (§columns).
2. A foreign key references exactly one row when its columns are NOT
   NULL, and no row when one of them is NULL, which only a column that
   carries `null` can be. Nothing is generated from this beyond the
   columns' nullability.

---

## Collected grammar (Part I)

The complete grammar in EBNF (ISO/IEC 14977), collected from the sections
above.

```ebnf
(* ===== Program structure ===== *)

program              = { element } ;

element              = table | table partial
                     | type decl | ref element | records element ;

(* ===== Table, Columns, Default Values, Indexes ===== *)

table                = "table", name, [ table settings ],
                       "{", table body, "}" ;
table body           = { column | indexes block | validators block
                       | partial injection | records block } ;
table settings       = "[", table setting, { ",", table setting }, "]" ;
table setting        = "model", ":", string ;

column               = name, column type, [ column settings ], newline ;
column type          = [ "[", "]" ], type name ;
type name            = [ identifier, "." ], identifier ;
column settings      = "[", column setting, { ",", column setting }, "]" ;
column setting       = "pk" | "null"
                     | "unique" | "index" | "autoincrement" | "required"
                     | "default", ":", default value
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
                     | "unique"
                     | "where", ":", expression literal ;

(* ===== Relationships ===== *)

ref element          = "ref", ":", ref endpoint, ">", ref endpoint,
                       [ ref settings ] ;
ref endpoint         = name, ".", column group ;
column group         = name | "(", name, { ",", name }, ")" ;
ref settings         = "[", ref setting, { ",", ref setting }, "]" ;
ref setting          = "delete", ":", ref action
                     | "update", ":", ref action
                     | "as", ":", name
                     | "reverse", ":", name ;
ref action           = "cascade" | "restrict" | "set_null" | "set_default" ;

(* ===== Types ===== *)

type decl            = "type", type name, storage type, [ "from", string ], newline ;
storage type         = column type ;

(* ===== TablePartial ===== *)

table partial        = "table_partial", name, "{", partial body, "}" ;
partial body         = { column | indexes block | validators block } ;
partial injection    = "~", name, newline ;

(* ===== Records ===== *)

records element      = "records", name, records columns,
                       "{", { record row }, "}" ;
records block        = "records", [ records columns ],
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

comment              = "//", { any char - newline } ;

identifier           = letter, { letter | unicode digit } ;
letter               = ? any character of Unicode category L (Letter) ?
                     | "_" ;
unicode digit        = ? any character of Unicode category Nd (Decimal Number) ? ;
digit                = "0" | "1" | "2" | "3" | "4"
                     | "5" | "6" | "7" | "8" | "9" ;

string               = single line string | multi line string ;
single line string   = '"', { sls char | escape sequence }, '"' ;
sls char             = any char - ( '"' | "\" | newline ) ;
multi line string    = '"""', mls body, '"""' ;
mls body             = { mls piece } ;
mls piece            = mls plain | escape sequence
                     | '"', ( mls plain | escape sequence )
                     | '""', ( mls plain | escape sequence ) ;
mls plain            = any char - ( "\" | '"' ) ;

escape sequence      = "\", escaped item ;
escaped item         = "\" | '"' | "n" | "t" ;

number               = digit, { digit },
                       [ ".", digit, { digit } ], [ exponent ] ;
exponent             = ( "e" | "E" ), [ "+" | "-" ], digit, { digit } ;

boolean              = "true" | "false" ;
null                 = "null" ;

expression literal   = "`", { any char - "`" }, "`" ;
punct                = "{" | "}" | "[" | "]" | "(" | ")"
                     | "," | ":" | "." | "~" | "*" | "/"
                     | "<" | ">" | "-" | "+" | "\" | "="
                     | "!=" | "<=" | ">=" | "?" ;
```

---

# Part II — The Project and Routing Layer

**Status:** Normative.
Part I specifies the schema core; this part specifies everything Volt
adds above it. Both parts cite sections by heading name, never by
number, and a semantic error carries the section it enforces as its
code, `spec/<anchor>` (`spec/scopes-and-routes`).

Every construct below is specified by (1) a grammar production in EBNF
(notation of Part I §notation), (2) an enumerated list of constraints, and
(3) a minimal example. The collected grammar appears in
[Collected grammar (Part II)](#collected-grammar-part-ii). The executable companion
is the conformance corpus in [`lang/conformance/snippets/`](../lang/conformance/snippets/):
files under `valid/` MUST be accepted by a conforming implementation,
files under `invalid/` MUST be rejected.

---

## Layers, files and the superset rule

1. A Volt source file has the extension **`.volt`**; a file with any
   other name is not part of any package. Whether a file happens to use
   only the schema core (Part I) or the full language follows from its
   content, never from its name.
2. **Layering rule.** The full language is a superset of Part I: every
   valid Part I program is a valid Volt file body, and a file body
   means the same at every layer. Part II adds the package clause and
   its own declarations and takes nothing away. The lexical grammar is
   Part I §lexical-structure, unchanged; the route-path and
   package-path productions name its `/` token `slash`:

   ```ebnf
   slash = "/" ;
   ```

   `/` is a token wherever it is not the start of a `//` comment, or
   of `/*`, which is an error (§comments). The rest-of-path wildcard
   is spelled `:name...` (§route-paths), so a path never writes `/*`.
3. Declaration order and file assignment carry no meaning; they fix
   only the order of generated output (the DDL, the route table) and of
   diagnostics (§projects-and-packages).

## Data and network layers

What Volt generates falls into two layers, told apart here and nowhere
else. These are not the layers of §layers-files-and-the-superset-rule,
which are Part I and the full language; Part II specifies parts of
both.

1. **The data layer** is the schema core (Part I), migrations
   (§migrations), groups (§groups), predicates (§predicates), selects
   (§selects-over-groups), validators (§validators) and changesets
   (§changesets). `volt gen` writes it into a package's data files,
   `volt_models.go`, `volt_queries.go`, `volt_dyn.go`,
   `volt_validate.go`, `volt_selects.go` and `volt_migrations.go` (with
   `volt_schema.sql` on request), over the data runtime `nao/rt`, whose
   values a program performs through the runner `nao/run`. A data
   file imports only `nao/rt`, the packages of the package's declared
   types (§types) and the standard library other than `net/http`; it
   names `database/sql` and `database/sql/driver` only for the
   `Scanner` and `Valuer` assertions of §types, never a connection.
2. **The network layer** is packages (§projects-and-packages), imports
   (§imports), pipelines (§pipelines), scopes and routes
   (§scopes-and-routes), resources (§resources) and datasets
   (§datasets). `volt gen` writes it into the route files,
   `volt_handlers.go`, `volt_router.go`, `volt_paths.go`,
   `volt_routes.go` and the client's `volt_client.go`, over the root
   `volt` package, the network runtime. A route file imports the
   standard library, the root `volt` package, the runner `nao/run` when
   it has a query route, and the data packages whose models and queries
   it names; the client imports the standard library, the root `volt`
   package, those data packages and the data runtime `nao/rt` when a
   method takes an optional parameter, whose `rt.Null[T]` it spells
   (§generated-client), and never the runner.
3. **The rule.** The network layer may use the data layer, never the
   reverse. The route files call a data package's queries and
   changesets, and the root `volt` package reads the data runtime's
   errors, a refused write and a row miss, to choose its status
   (§formats). No data file and no
   package under `nao/` reaches the root `volt` package, the route
   generator `gen/router`, the generation driver `gen/model`, the
   language server or the tool, and `nao/rt` imports the standard
   library alone, never `net/http`. The front end `lang/` checks both
   layers; the names of the data layer are decided once, in
   `nao/naming`, which the data generator renders and the front end and
   the language server read, so no checker imports a generator. A layer
   contract test (`cmd/volt/layers_test.go`) fails the build when any of
   this breaks.
4. **The data layer alone.** A package whose files declare no scope
   has no routes, since resources and datasets live only in scopes
   (§scopes): it generates its data files alone, `volt gen` removes
   the route files it wrote for the package while it had a scope, and
   a program built on it links nothing from Volt but `nao/rt` and the
   runner `nao/run`. Beside
   the package clause (§projects-and-packages), the one declaration of
   the network layer such a package can carry is a pipeline, which no
   scope pipes through and which generates nothing.

## Projects and packages

```ebnf
package clause = "package", name, newline ;
```

1. A **project** is a Go module: the directory tree rooted at the
   nearest ancestor directory containing **`go.mod`**. Its
   `module` directive is the project's module path; every other
   directive is Go's business and ignored. There is no second manifest.
2. Every `.volt` file MUST begin with a package clause: its first
   declaration, at most one per file. The name is an identifier, and
   the clause takes no settings list; the doc comment above it
   documents the package (§doc-comments). The name `volt` is reserved: generated files import the
   volt runtime under that qualifier (§generation-contract-informative), so a package of that name
   is an error.
3. A **package** is a directory: all `.volt` files in one directory
   MUST declare the same package name, and share one namespace per
   element kind (tables, types, partials, pipelines, predicates, and the
   reverse-URL helpers and client methods of the routes, each per Part I
   §namespaces-and-uniqueness extended with §pipelines, §predicates and
   §route-names-and-reverse-urls).
4. In every directory other than the project root, the package name
   MUST equal the directory's base name, and the directory path is a
   package path (§imports). The root directory's package may
   take any name.
5. **Layout invariance.** Splitting or merging the `.volt` files of a
   package, or reordering declarations, MUST NOT change the meaning of
   the package. Implementations MUST process a package as the
   concatenation of its files in file-name order, and route expansion
   (§route-conflicts) follows declaration order within that concatenation.
6. Directories whose name begins with `.` or `_`, and directories named
   `testdata` or `node_modules`, are not part of the project: the first
   three are Go's own exclusions, `node_modules` is Volt's. Neither is any subdirectory containing its own `go.mod`
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
   declares a bare `type` writes no Go to a `DIR` other than its own
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

table users {
	id int [pk]
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
   import entries; an empty block is an error. It may stand anywhere
   after the package clause, and an import in one file serves every file
   of the package (rule 5): like every declaration, its position carries
   no meaning (§projects-and-packages).
2. A package path names a package by its directory path from the
   project root, `/`-separated, with no `./`, no `..`, and no interior
   whitespace; its segments, and the alias, are identifiers of ASCII
   letters, digits and underscores (§route-paths). The path MUST name an
   existing package of the project. Every package's directory path,
   imported or not, is held to the same segments, since it is also the
   package's Go import path: a package in a directory `żółw` is an
   error, although its name, like any name, may be Unicode.
3. The imported package is referenced through its **qualifier**: the
   alias when given, else the last path segment. All cross-package
   references are qualifier-prefixed (`db.User`); nothing is ever
   imported into the local scope. A `migration` is not an importable
   element: it belongs to the package whose tables it migrates, and
   nothing of another package may name it (§migrations).
4. **A package names its own declarations bare**, as a Go package does.
   A dotted reference names an import through its qualifier (a query
   route's handler, a `resources` table, a `dataset` select) or the
   runtime as `volt.` (a plug, the event stream; §pipelines,
   §event-routes). A plug, an error handler and a validator never name
   an import (§pipelines, §scopes, §validators), and the package's own
   name qualifies nothing. A qualifier that is neither is an error, and when it is the
   package's own name the error names the bare form: inside package
   `app`, `app.Errors` is written `Errors`.
5. Within one package (across all its files): importing the same path
   twice is idempotent, but with two different qualifiers it is an
   error; two different paths MUST NOT share a qualifier; a package MUST
   NOT import itself; **every import MUST be used**: an import whose
   qualifier is never referenced is an error. An import MAY take the
   package's own name as its qualifier, since that name qualifies
   nothing else (rule 4).
6. **The generated files' own names.** A qualifier MUST NOT be
   `context`, `http`, `rt`, `run`, `time` or `volt`: the route files
   and the client import the standard library's `context`, `net/http`
   and `time`, the data runtime `nao/rt`, the runner `nao/run` and the
   root `volt` package under those names (§data-and-network-layers),
   and they import a data package under its qualifier, so a project
   package under one of those names would give a generated file two
   imports of one name. The error names an alias to write: the entry
   `rtdb rt` imports a package `rt` under `rtdb`. A package that
   routes its own tables imports nothing, and its client names it as
   §generated-client says.
7. **Import cycles are errors.** The package import graph MUST be
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
pipeline = "pipeline", name, "{", { plug }, "}" ;
plug     = "use", go ref, newline ;
go ref   = name, [ ".", name ] ;
```

1. A `pipeline` is a named, ordered middleware list. `pipeline` names form
   one namespace per package and MUST be unique. A pipeline body
   contains only plug lines; it MAY be empty.
2. A plug references Go middleware of type
   `func(http.Handler) http.Handler` by name: `volt.<Name>` (the volt
   runtime) or `<Name>` (a function of the containing package, named
   bare, §imports). Referencing an *imported* package's function is
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
3. A pipeline contributes its plugs in declaration order, outermost
   first, composed statically in generated code; pipelines compose as
   §scopes chains them.

```volt fragment
pipeline api {
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
param type = "int" ;
```

1. All tokens of a route path are **contiguous**: interior whitespace
   ends the path. A bare `/` is the root path. A non-root path MUST NOT
   end with `/` and MUST NOT contain an empty segment (`//`): routes
   match exactly (§scopes), so a trailing slash would spell a silently
   different pattern.
2. A `:name` segment captures one path segment as a parameter. The
   parameter name MUST be a valid, non-keyword Go identifier, MUST NOT
   be one of the reserved names `w`, `r`, `opts`, `volt`, `cs`, `c`,
   `ctx`, `body` (they appear in generated signatures, the router's,
   the handlers' and the client's, and would be shadowed), and MUST be unique within the
   route's full path (scope prefixes included). The rules of this
   section hold for a `scope` prefix where it is written: its errors are
   reported once, an empty `scope`'s included, and nothing beneath a
   prefix in error registers, a `:name(string)` segment excepted
   (clause 3); a route parameter repeating a prefix parameter is
   reported at the route.
3. A bare `:name` is a `string` parameter and `:name(int)` an `int`
   one: the closed type set is `int` and `string`, the Go types the
   model generator emits for routable primary keys, and the annotation
   names a type only when it is not the string, so `int` is the one
   type written. `:name(string)`, the word in any case, is an error
   naming the bare form, in a route and in a `scope` prefix alike,
   reported once where it is written; it would spell the bare
   parameter a second way. The segment is then read as the bare
   parameter, so the route, or every route beneath such a prefix,
   still registers and meets §route-conflicts.
   **Types shape handler signatures, never matching**: a request
   segment that fails to parse as the declared type is that route's
   404, not a fallthrough to a sibling route.
4. `:name...` captures the rest of the path (a **wildcard**), always as
   `string`. It MUST be the last segment of the full path, MUST NOT
   carry a type, and MUST NOT appear in a `scope` prefix. (Spelling
   rationale: §layers-files-and-the-superset-rule.)
5. Grammar note: a path is assembled from ordinary tokens; the
   contiguity rule of clause 1 is what makes it one lexical island.
6. Every segment name, literal, parameter or wildcard, MUST be an
   identifier of ASCII letters, digits and underscores. Segment names flow verbatim into registration patterns
   and generated Go; the closed character set is what makes that safe
   (and is why `{`/`}` can serve as the collision-free conflict markers
   of §route-conflicts). The same constraint applies to the `resources` table
   name (§declaration), to the identifiers of pipeline names and plug
   references (§pipelines), to route handler names (§handler-references), and to import
   aliases and path segments (§imports).

### Routes

```ebnf
route = verb, route path, handler ref, [ settings ], [ load block ], newline ;
verb  = "get" | "post" | "patch" | "delete"
      | "options" | "head" | "any" ;
```

1. Routes appear only inside a `scope` body. The verb is a contextual
   keyword, lowercase (§keywords); `any` registers the route for every
   method (its registration carries no method). A controller route
   takes every verb; a query route takes the one its method fixes
   (§query-routes).
2. `put` is not a verb. PATCH is the partial update, the one a
   generated update performs (§query-routes), and PUT, which means
   replace in HTTP, would be a second spelling of it. A route line
   opening with `put`, the word in any case, is an error naming the
   verb the line takes: the one its method fixes on a query route
   (§query-routes), `get` on an event route (§event-routes), and
   `patch` otherwise. The line is read as that verb, so the error is
   reported once, and it registers nothing. Outside a `scope` a `put`
   line is the error of clause 1.
3. A route's **full path** is the enclosing scopes' prefixes followed
   by its own path, in nesting order.
4. Registration semantics are `http.ServeMux` (Go ≥ 1.22) exactly:
   parameters register as `{name}`, wildcards as `{name...}`, and a
   root full path as `/{$}`. Matching is exact: Volt never registers
   subtree patterns. ServeMux serves HEAD for a GET pattern, so a
   `get` route also answers HEAD, and a `head` route on the same path
   takes HEAD apart from it.

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
   package, or `Query`, a generated query of the package itself
   (§query-routes), named bare (§imports). The
   runtime's own handlers are `volt.<Name>` (§event-routes).
2. Every distinct controller becomes one generated interface; every
   distinct action one method with the scope loads of its scopes
   first (§scope-loads), the route's typed parameters appended after
   them, its body after those (§request-bodies) and its loads last
   (§route-loads), returning
   `volt.Response` (§requests-and-responses); the type that implements
   it is an empty struct (`lint.md`, `handlerreceiver`).
3. Two routes MAY share `Controller.Action` only with identical scope
   loads (§scope-loads), identical parameter signatures (names, types
   and wildcard-ness, in order), the same body type or none
   (§request-bodies) and identical loads (names and types, in order;
   §route-loads). At most one of the routes that share an action carries
   a doc comment (§doc-comments); its text documents the action's method
   (§doc-comments-in-generated-code), and a second documented route is an
   error naming the first.
4. The generated `Controllers` manifest is one namespace: a controller
   MUST NOT take the name of a field the manifest holds for a data
   the query routes run on (`DB`; §query-routes) or for the event broker
   (`Events`; §event-routes). The error names the field.

### Requests and responses

A handler is a pure function from a request value to a response value
(§values-and-effects): it reads the request, describes what the
database should do and what the client should receive, and the
generated shim does both.

```go
type Request struct { /* unexported */ }
func (r Request) Method() string
func (r Request) Path() string
func (r Request) Route() string            // the matched ServeMux pattern
func (r Request) PathValue(name string) string
func (r Request) Query() url.Values
func (r Request) Header() http.Header
func (r Request) Body() []byte             // read once by the shim, at most MaxBodyBytes
func (r Request) Now() time.Time           // the clock, captured once

type Response struct { /* unexported: a done response, or a continuation */ }
func Render(v any) Response                // 200, JSON or GOB by Accept (§formats)
func RenderStatus(status int, v any) Response
func Created(v any) Response               // 201
func NoContent() Response                  // 204
func Redirect(url string) Response         // 303
func Reject(status int, msg string) Response   // a Problem body (§formats)
func Fail(err error) Response              // the error handler chooses (§scopes)
func File(fsys fs.FS, path string) Response
func HTML(b []byte) Response
func Stream(fn func(io.Writer) error) Response   // the one escape
func (r Response) WithStatus(code int) Response
func (r Response) WithHeader(k, v string) Response
func (r Response) WithCookie(c http.Cookie) Response
func (r Response) WithDetails(errs []rt.FieldError) Response

func Then[T any](e rt.Effect[T], next func(T) Response) Response
func (r Response) Rescue(on func(error) Response) Response
```

1. **The request value.** The shim builds one `volt.Request` per
   request: the method, the path, the matched route, the path
   parameters the route captures, the query string, the headers, the
   body, read once up to `volt.MaxBodyBytes` (a larger body is a 413
   before the handler runs), and the time, captured once, which
   `Now()` returns. It carries no writer and no context; the shim holds
   both. Nothing a handler reads from it changes between two reads.
2. **The response value.** A response is either **done**, a status,
   headers, cookies and a body, or a **continuation**: `Then(e, next)`
   names an effect of `nao/rt` (§generated-query-surface) and the
   function that turns its result into the next response. The shim is
   a loop: perform the effect, call `next`, repeat until done, then
   write. `Then` is the only way a handler reaches the database, and
   a handler calls nothing that performs one. Each continuation's
   effect is performed on its own, so two continuations are never one
   transaction: writes that must commit together, a row and the rows
   that carry the key its insert assigned among them, are one effect,
   an `rt.Atomic` over `rt.Then` (§generated-query-surface).
3. **Failure.** An effect that fails, and a `Fail(err)` a handler
   returns, go to the route's error handler (§scopes), which answers
   404 for a row miss, 409 or 422 for a refused write, an `HTTPError`'s
   own status, and 500 otherwise (§formats). `Rescue` on a continuation
   names the function that answers for its effect's failure instead,
   for a route that must decide on a miss; on a done response it does
   nothing. It covers that one effect: the function receives the
   error, a timeout's 504 among them (§route-loads), and its response
   goes on through the shim as `next`'s would, a `Fail(err)` to the
   error handler; a step `next` returns is rescued only by a `Rescue`
   of its own. A scope load's `Rescue` does the same for the load's
   effect (§scope-loads). An effect that fails because the request's
   context was cancelled, the client having closed the request, fails
   with a 499 the same way, the status proxies log for it, since no
   standard status says it: the error handler answers it as an
   `HTTPError`, logging nothing as a handler error, and nobody reads
   the answer.
   `Reject` is a Problem body of the handler's own, with `WithDetails`
   for the field errors of a changeset it judged.
4. **Bodies.** `Render` and `RenderStatus` write a value in the
   negotiated format, and a nil `v` writes the status and no body
   (§formats); `File` serves a file of an `fs.FS` as `net/http`
   does; `HTML` writes bytes as `text/html`; `Redirect` writes a
   `Location`; `Stream` hands the body to a function, the one response
   that is not a value, which the lint annex counts (`lint.md`,
   `streamcount`); a failure inside it is logged, the response being
   committed. `WithHeader` and `WithCookie` return a response with the
   addition and leave their receiver as it was.
5. **Determinism.** Two calls of a handler with one request value
   return the same response value, and `volt_handlers.go` declares
   `ControllersDeterministic(c Controllers, r volt.Request) []string`,
   which calls every action twice with zero-valued parameters, a
   changeset body as the fresh changeset `<Model>New()` (§changesets)
   since a changeset no constructor made refuses a setter, and
   names the ones whose responses differ (`volt.Same`: the same status,
   headers, cookies and body, or continuations of the same effect;
   streams compare as the same). Functions cannot be compared, so a
   continuation compares by its effect alone, never by `next`, as
   `rt.Same` compares two effects, and an effect that wraps others by
   its kind and what it wraps: an `rt.Then` by being a `Then` and by
   its first effect, never by the one its `next` would make, and an
   `rt.Map` and an `rt.Atomic` by their kind and their inner one, so an
   `Atomic` over `e`, a `Map` over `e` and `e` itself are three effects;
   a statement compares by its kind, text and arguments, an `Exec` by
   its `One` and `Model` as well and a `Write` with no statement by the
   `Row` it yields, an `rt.Pure` by its value as a done response's body
   does, and an `rt.Fail` by its error, as a `Fail(err)` response does
   (§generated-query-surface). Every value, an argument, a row, a body
   and an error among them, compares with `rt.Equal`, which is
   `reflect.DeepEqual` except that a NaN equals a NaN, so a value that
   holds no function is always the same as itself and `1` is not
   `"1"`; two errors of one message and two statuses are two. The rule
   compares spellings, not what they perform: two spellings of one
   computation, a `Map` and a `Then` whose `next` returns an `rt.Pure`
   among them, are two effects, which never misleads the check, since
   a handler called twice spells its effect the same way both times. An
   application's test calls it with its controllers and asserts an
   empty list.
6. **What keeps a handler pure** is the generated interface, which
   fixes every parameter, and the vet rules of `lint.md`:
   a controller is an empty struct (`handlerreceiver`), a file that
   declares a handler or a scope-load function (§scope-loads) imports
   neither the runner nor `database/sql`,
   `net`, `os`, `os/exec` nor a random source (`handlerimports`), it
   reads no clock but `r.Now()` (`handlerclock`), a routes package holds
   no package state and starts no goroutine (`packagestate`,
   `goroutine`), a function the runner calls, the `next` of an
   `rt.Then` and the `f` of an `rt.Map` or an `rt.ParallelMap`, writes
   no variable it captures (`capturedwrite`), and every `Stream` is
   reported (`streamcount`). Not caught: reflection, `unsafe`, cgo, a
   package imported under another name, a call through a function
   value, and a write such a function makes through a method or a
   function it calls.

### Request bodies

```ebnf
query call = query ref, "(", [ arg, { ",", arg } ], ")" ;   (* with query ref, the value of body:, a setting value (§settings-lists) *)
query ref  = name, [ ".", name ] ;
arg        = name | literal ;
```

```volt fragment
scope / {
	patch /settings/:id(int) Settings.Save  [body: db.SettingUpdate(id), only: (unit, note)]
	post  /metrics           Metrics.Create [body: db.MetricCreate]
	post  /login             Session.Login  [body: LoginForm]
}
```

A controller route may bind its request body with the `body:`
setting: the shim decodes the body before the handler runs, and the
handler receives a value, never bytes. The setting takes two forms,
because they bind two different things.

1. **A changeset.** `body:` names a write of a data package, `Create`
   or `Update` (§generated-query-surface), written `pkg.Query` for an
   imported data package or `Query` for the package's own tables, as a
   query route names one (§query-routes). An update takes its key in
   parentheses, one argument per key column: a path parameter of the
   route whose type spells the key's Go type, or a literal that does
   (§route-loads). A create takes no argument, its parentheses empty
   or absent. The shim does what a query route's shim does up to the
   write (§query-routes): it decodes the body by `Content-Type` into
   the model's Params (§formats), loads the row of an update, casts
   with the write's allow list (§changesets) and validates
   (§validators). The changeset, valid or not, is what the handler
   receives; the handler reads `Valid()` and `Errors()`, adds findings
   of its own, and describes the write with the function the body
   named, `db.SettingUpdate(body)`, or does not. A body that does not
   decode is a 400 or a 415 before any row loads (§formats); a missing
   row of an update is a 404 through the error spine, as a load's
   would be (§route-loads).
2. **`only:`** narrows the cast to the listed columns, a column list
   `(a, b)` naming each once, every one a column the write casts
   (§changesets). `only:` without `body:`, or with a struct body, is
   an error.
3. **A struct.** `body:` names an exported struct type declared in
   the routes package's Go files; the body decodes into it with the
   rules of `volt.Decode` (§formats): a field the struct does not
   declare is a 400 naming it.
4. **Signature.** The handler receives the value as the parameter after
   the route's parameters and before its loads (§route-loads), named
   `body` and typed as the changeset type, qualified by the package
   unless the write is the package's own, or as the struct:
   `Save(r volt.Request, id int, body db.SettingChangeset)
   volt.Response`. Two routes sharing an action share its body type
   (§handler-references); `body` is a reserved name a path parameter
   cannot take (§route-paths) and a load cannot be named (§route-loads).
5. **Order.** The body is bound before the loads run: a body that does
   not decode loads nothing, and the row of an update loads first.
6. A body on a query route or an event route is an error: their
   handlers are generated, and a query route's write decodes its own
   body (§query-routes).

```go
func (Settings) Save(r volt.Request, id int, body db.SettingChangeset) volt.Response {
	if !body.Valid() {
		return volt.Reject(422, "check the fields").WithDetails(body.Errors())
	}
	return volt.Then(db.SettingUpdate(body), func(db.Setting) volt.Response {
		return volt.NoContent()
	})
}
```

### Route loads

```ebnf
load block = "{", { load }, "}" ;
load       = name, query call, newline ;
```

```volt fragment
scope / {
	get /charts Pages.Charts [timeout: 5] {
		metrics  db.MetricOrdered()
		settings db.SettingGet(1)
	}
}
```

A controller route may carry a **load block** before its newline:
the rows the handler needs, loaded before it runs and handed to it as
parameters, so the handler reads them and never asks for them.

1. A load names a generated read of a data package
   (§generated-query-surface): a select function, `Get` or `List`,
   written `pkg.Query` for an imported data package or `Query` for the
   package's own tables, as a query route names one (§query-routes). A
   write, `Create`, `Update` or `Delete`, is an error, and so is a
   name that is no generated query. The load's name is the parameter
   the handler receives the value under: a lower-case Go identifier,
   so no Go keyword (`range`, `func` and the rest), that is not a
   reserved name (§route-paths), not `err` or `ok`,
   which the shim declares beside it, not a name the route files
   import a package under (`context`, `http`, `rt`, `run`, `time`,
   `volt`; §imports) and not a predeclared Go identifier (`string`,
   `int`, `len`, `nil` and the rest of Go's universe block), either of
   which the shim's closure taking the value would shadow, not shaped
   like a generated
   local (`volt` and an upper-case letter), not the package's name or
   an import qualifier (§imports), not a path parameter of the route
   and not another load of it or of a scope above (§scope-loads).
2. **Arguments** are given in the function's order and number, each a
   name or a literal. A name that is a path parameter of the route
   binds it, and the parameter's type MUST spell the function's Go
   type, as for a query route; a wildcard cannot bind one, nor can a
   path parameter bind a list parameter. A name that is another load
   the route declares, before this one or after it, or a scope load
   above it (§scope-loads), and no path parameter, is an error: no load
   binds another load's value, and the
   name, read from the query string as any other name is, would let the
   client choose the value, the signed-in user's id among them; a
   handler that needs a row keyed by a loaded value reads it with
   `Then`. The load's own name is no other load's, so an argument may
   share it and binds the query key of that name: `title
   db.PostTitled(title)`. Any other name binds from the
   query string by that name, a list parameter from the repeated key,
   with the types the query string carries (§query-routes); a missing
   or unparseable value is a 400 naming the key, before any of the
   route's loads runs. One key may bind several arguments, of one load
   or of several, and each binds by its own parameter's rule, so a
   missing key that one of them requires is a 400 whatever else it
   binds; every argument a key binds has one Go type, the one the
   generated client sends it as (§generated-client), so a key two
   arguments bind as two types is an error. An optional parameter
   (§predicates) binds from the query string only, absent when the key
   is missing, as a query route binds it (§query-routes). A path
   parameter or a literal, which is always present, cannot bind it,
   and the error names the fix: a query-string key, or a second select
   whose parameter is required, which the path parameter or the
   literal then binds. An optional list parameter cannot be
   loaded at all, whatever its argument, for the reason a query route
   cannot route one (§query-routes). A literal MUST spell the parameter's Go type: a
   string literal a `string`, a number an `int` when it is whole and
   a `float64` otherwise, `true` or `false` a `bool`. A parameter of a
   type the query string cannot carry, a declared type (§types) among
   them, is spelled by no literal and no path parameter either, so no
   load binds it, optional or not: whatever its argument, the load is
   an error naming the parameter and its type. A number
   binds in canonical Go form, as a predicate's literal does
   (§predicates): `010` binds `10`, never the octal `8` Go would read
   in it. A select's `limit:` parameter (§selects-over-groups) takes a
   value of at least 1: a literal below 1 is an error, since every
   request would be refused; a value below 1 from a path parameter is
   a 400 naming the key once the path parameters parse, before the
   scope loads (§scope-loads), and one from the query string is a 400
   naming the key where it binds, before any of the route's loads
   runs.
3. **Order and the shim.** Loads run in declaration order before the
   handler, each a continuation of the shim (§requests-and-responses),
   and every argument is bound before the first runs. The handler's
   signature is generated from them: after the route's parameters and
   its body (§request-bodies), one parameter per load named as the load
   is, typed as the query yields,
   the row type for `Get`, a slice of it for `List` and a select,
   qualified by the package unless the query is the package's own:
   `Charts(r volt.Request, metrics []db.Metric, settings db.Setting)
   volt.Response`. Two routes sharing an action share its loads
   (§handler-references).
4. **Failure.** A load that fails goes to the error spine as an effect of
   the handler would (§requests-and-responses): a missing row is a 404,
   and the handler never sees it. A route that must decide on a miss
   loads nothing and uses `Then` with `Rescue`.
5. **`timeout`.** A route setting, a positive integer of seconds,
   bounds the whole chain, loads and the handler's own effects, with
   one deadline from the request's arrival; an effect the deadline
   cuts off fails with a 504, which goes to the error spine unless a
   rescue answers for that effect (§requests-and-responses,
   §scope-loads). The runner sees the deadline at each statement and
   while a transaction waits for the write lock, but a single statement
   outside a transaction that finds the lock held waits under SQLite's
   busy handler, which sees no deadline, and fails with the 504 only
   when that wait ends (§generated-query-surface, the runner): a write
   that must answer at the deadline while another connection writes
   runs in an `Atomic`. A query route takes it too; an event route,
   whose stream the runtime serves, takes none.
6. A load block on a query route or an event route is an error: their
   handlers are generated.

### Scope loads

```ebnf
scope load = "load", name, go ref, newline ;
```

```volt fragment
scope /me [pipe: app] {
	load user CurrentUser
	get /       Me.Show
	get /orders db.OrderList
}
```

```go
type Load[T any] struct { /* unexported: an effect, or a halt */ }
func LoadWith[T any](e rt.Effect[T]) Load[T]
func Halt[T any](resp Response) Load[T]
func (l Load[T]) Rescue(on func(error) Response) Load[T]

func CurrentUser(r volt.Request) volt.Load[db.User] {
	id, err := strconv.Atoi(r.Header().Get("X-User"))
	if err != nil {
		return volt.Halt[db.User](volt.Reject(401, "sign in"))
	}
	return volt.LoadWith(db.UserGet(id)).Rescue(func(err error) volt.Response {
		if errors.Is(err, rt.ErrNotFound) {
			return volt.Reject(401, "sign in")
		}
		return volt.Fail(err)
	})
}
```

A scope may **load** a value for every route beneath it: a function
of the routes package decides, from the request alone, what the
routes need and whether the request goes on.

1. A scope load names the parameter and a Go function of this
   package, written by its bare name (§imports), whose shape the
   checker holds: `func(volt.Request) volt.Load[T]`. `volt.Load[T]`
   is a sealed sum built by `volt.LoadWith(e)`, an effect of `T` the
   shim performs, or `volt.Halt[T](response)`, a done response that
   ends the request before anything beneath runs; the zero `Load`,
   which decides nothing, is an error through the error spine. The
   function is a pure function of the request like a handler
   (§requests-and-responses), and the Go rules of `lint.md` read it so.
2. `T` is any Go type the file spells; a package it names by a
   qualifier is an import of this package (§imports), which the
   generated handlers file imports under the same qualifier. The
   load's name obeys the rules of a route load's (§route-loads), and
   no route beneath uses it for a path parameter, a route load, another
   scope load, or a parameter a query route or a dataset beneath binds
   from the query string (§query-routes, §datasets): such a route drops
   the load's value (rule 3), so the key would let the client choose
   the value the name stands for, the signed-in user's id among them.
   The function a scope load names is not the name of a load declared
   before it in its scope or of a load of a scope above it, which holds
   that load's value where the shim calls the function; the load's own
   name and a later load's are free, since the shim calls the function
   before it holds either value: `load current current` calls
   `current` and then holds its value as `current`.
3. **Order and the shim.** `scope` loads run for every route beneath
   the scope, controller and query routes alike, outermost scope
   first and in declaration order within a scope, after the path
   parameters parse, a path parameter that binds a select's `limit:`
   checked with them (§route-loads, §query-routes), and before the body
   (§request-bodies), the query string and the route loads
   (§route-loads); a halt answers with its response, an
   effect that fails goes to the error spine (§requests-and-responses)
   unless the load rescues it.
   A controller route's handler receives the values as its leading
   parameters, after `r` and before the route's parameters, nested
   scopes appending: `Show(r volt.Request, user db.User) volt.Response`.
   Two routes sharing an action share its scope loads
   (§handler-references). A query route runs them for the halt and
   drops the values. An event route (§event-routes) runs no load, so
   one beneath a scope load is an error.
4. **Failure.** `l.Rescue(on)` is the load `l` with `on` answering for
   its effect's failure instead of the error spine: `on` receives the
   error, a timeout's 504 among them, nothing beneath runs, and the
   response `on` returns answers the request, a `Fail(err)` going on to
   the error handler. A load so answers a miss of its own, a 401 for a
   user no row holds, and fails with every other error. The rescue
   covers the load's effect alone: a later scope load, the body
   (§request-bodies), the route's loads (§route-loads), a query route's
   query and the handler's own continuations fail to the error spine as
   they would without it. On a `Halt` and on the zero `Load` it does
   nothing, as `Rescue` does nothing on a done response
   (§requests-and-responses); a second `Rescue` replaces the first.

### Query routes

A **query route** binds a route to a generated query function of a data
package — an imported one, or the package itself when it declares the
tables — instead of to a controller. Volt generated the
method, so it knows the parameters and the row type, and it writes the
handler: bind, call, render. Nothing is dispatched by name at runtime.

```volt fragment
package app

import (
	db
)

scope /api [pipe: api] {
	get    /users            db.UserList
	get    /users/:id(int) db.UserGet
	post   /users            db.UserCreate
	patch  /users/:id(int) db.UserUpdate
	delete /users/:id(int) db.UserDelete
	get    /picked           db.UserPicked      // Select picked for users where id in :ids
}
```

1. **Which functions qualify.** A select function, `<Model><SelectName>`
   per member (§selects-over-groups), or a default CRUD function of a table:
   `<Model>Get`, `<Model>List`, `<Model>Create`, `<Model>Update`,
   `<Model>Delete`, each only where the query generator emits it
   (`Get`/`Update`/`Delete` need a primary key, `Update` a non-key
   column). Any other name is an error listing what qualifies; a name
   differing only in case names the right spelling. The function fixes
   the route's verb: a read, `Get`, `List` or a select function, takes
   `get`, which ServeMux also serves for HEAD (§routes); `Create` takes
   `post`; `Update` takes `patch`, the partial update; `Delete` takes
   `delete`. Any other verb on a query route, `head`, `options` and
   `any` included, is an error naming the right one, so a GET never
   writes and the route matches what resources and datasets generate
   for the same method; a controller route keeps every verb (§routes).
2. **Binding.** Each parameter of the function, in signature order,
   binds from exactly one source, decided at generation time:
   - a **path parameter** of the route with the same name binds it.
     Its declared type MUST spell the function's Go type
     (`:id(int)` for `id int`, a bare `:slug` for `slug string`); a
     type a segment cannot carry (§route-paths) goes in the query string.
     A wildcard cannot bind a query parameter;
   - a **changeset** parameter (`<Model>Create`, `<Model>Update`)
     binds from the **request body**: the body decodes as
     `<Model>Params` by `Content-Type` (Formats; §changesets, `Params`)
     and is cast into a changeset. `Create` casts into `<Model>New()`
     allowing every column but the database-assigned key (§columns),
     which the database assigns, so a client cannot choose an id, while
     a shared key that is also a foreign key is cast like any column,
     since its value is the referenced row's, and a body that leaves it
     out is a 422 with `not_null` on it (§changesets); `Update` loads
     the row by the key path parameters with `<Model>Get` (a miss is a
     404), casts into its `Changeset()` allowing every non-key column,
     and the route MUST carry every key column as a path parameter.
     The body is decoded before the row is loaded, so a malformed body
     is a 400 whether or not the row exists. A write route always takes
     a body, `{}` when nothing is set: a table whose only column is the
     database-assigned key still needs `{}`. An update is partial: nothing
     the body leaves out is reset, which is why it is routed with
     `patch` (clause 1);
   - a **list parameter** (§predicates) binds from the **repeated query
     key** of its name; absent means the empty list;
   - an **optional parameter** (§predicates) binds from the **query
     string** by name as `rt.Null[T]`: absent when the key is missing,
     present with the parsed value otherwise, and an unparseable value
     is a 400 that names the parameter. A path segment is always
     present, so a path parameter of its name is an error, which names
     the fix: pass it in the query string, or route a second select
     whose parameter is required. An optional list parameter is not
     routable: a missing key and an empty list cannot be told apart in
     a query string, and an absent list matches every row where a
     present empty one matches none, so a route to its select is an
     error; a Go caller, which can spell both, calls the select itself;
   - every other parameter binds from the **query string** by name,
     parsed to its type; a missing or unparseable value is a 400 that
     names the parameter.
   A select's `limit:` parameter (§selects-over-groups) is an `int`
   and binds as one, from a path parameter of its name or the query
   string; a value below 1 is a 400 that names the parameter, since
   SQLite reads a negative limit as none: from a path parameter once
   the path parameters parse, before the scope loads (§scope-loads),
   and from the query string where it binds, before the query runs.
   A default or a most for the limit is a handler's to write: a
   controller route reads the key and calls the select with the limit
   it decides.
   A parameter binds from the query string only when its type is
   `string`, `bool`, `int`, `float64` or `time.Time`, a slice of one
   of those (a list parameter), or an `rt.Null` of one of those (an
   optional parameter); a select with a parameter of any other
   type, a declared type (§types) included, is not a query route, and
   the route is an error. A `float64` takes no NaN, which SQLite binds
   as NULL, so a present optional value would read as absent
   (§predicates): `NaN`, in any case, is an unparseable value, a 400
   like any other, wherever a `float64` binds from the query string,
   a list item and a load's argument (§route-loads) included. A path
   parameter the function does not take is an error: the route would
   capture a value nothing consumes; the key of an `Update` is
   consumed by the load.
3. **Response.** The result renders in the negotiated format
   (Formats): rows or a row with status 200, a created row with 201,
   a delete with 204 and no body. A query's row miss (`rt.ErrNotFound`) is a 404 through the error
   spine. A
   refused write is a `*rt.Invalid`, which carries no HTTP status
   (§changesets, `rt.Invalid`); the error spine gives it one (§formats):
   409 when a unique violation is among its field errors, 422 otherwise,
   itemized per field, an unknown key of the body among the fields. Any
   other error is the spine's 500.
4. **Names.** The helper name (§route-names-and-reverse-urls) defaults to the scope name
   prefixes followed by the function name, and only the `get` routes of
   reads carry one: writes have no reverse URL, as with resources
   (§the-action-table). `volt routes` and the route table show the reference as
   written (`db.UserGet`; a query of the package itself unqualified,
   `UserGet`) in place of `Controller.Action`.
5. **Wiring.** The generated `Controllers` manifest (§handler-references) holds
   `DB run.DB`, the one database every effect of the package runs on,
   which the application opens with `run.Open`
   (§generated-query-surface). The shim performs the query as a
   continuation (§requests-and-responses). No interface is generated
   for a query route.
6. **Own tables.** A package that declares tables MAY route them
   itself, so one directory holds schema, routes and handlers.
   `Query` names its own generated query;
   `resources t [default]` (§default-handlers) and `dataset s` (§dataset-declaration) resolve in
   the package when bare, as every declaration of the package is named
   (§imports). Nothing is imported, the
   generated handlers name the package's changesets bare, and the
   router's constructor is `NewRouter`, so it never collides with the
   query layer's `New`.

### Formats

The wire format of a query route is negotiated by HTTP headers and
never spelled in the URL, so a path parameter can never be mistaken
for a format suffix.

1. **Responses** follow `Accept`: the first listed media type that is
   `application/json`, `application/x-gob`, `application/*` or `*/*`
   decides; an absent header means JSON; a header offering neither is
   a 406 naming both. A query route's shim negotiates before it runs
   anything: its 406 decodes no body, loads no row and writes nothing.
   A controller route's answer is the handler's, so rule 4 says when
   it negotiates.
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
   every other language see. GOB carries no empty slice, so empty bytes
   arrive as nil, which is the same value, empty bytes, in either kind
   of `[]byte` column, and a NULL arrives as an invalid `rt.Null`;
   re-rendered as JSON, such a nil plain `[]byte` is `null`, not `""`
   (§nullability).
4. The runtime exposes the same rules to hand-written handlers as
   `volt.Render`, `volt.RenderStatus`, `volt.Created`, `volt.Reject`
   and `volt.Decode` over the request value's body
   (§requests-and-responses), so a controller route speaks both
   formats without repeating them. A controller route negotiates when
   it renders, since its answer may be HTML, a redirect, a file or a
   stream that neither format covers. Its shim decodes the body, loads
   an update's row and runs the route's loads before the handler
   starts (§request-bodies, §route-loads), all of them reads, so a body
   that does not decode is still a 400 or a 415 whatever `Accept`
   says. A handler that writes and then renders a format calls
   `volt.Negotiate` before its first write, so its 406 writes nothing.
5. **Error bodies** are structured: the default error handler writes a
   `volt.Problem` (status, message, and details: check, columns,
   message, when the error itemizes them) in the negotiated format,
   JSON when the client offered nothing usable. `volt.ProblemOf` finds
   it and is where the network layer chooses a status
   (§data-and-network-layers): an `HTTPError` keeps its own; a refused
   write, the data runtime's `*rt.Invalid` (§changesets), is 409 when a
   `unique` field error is among its fields and 422 otherwise; a query's
   row miss is 404; any other error has none and is the default
   handler's 500, `rt.ErrLimit` among them, a limit below 1 the
   application computed (§selects-over-groups). A rendered value is encoded before anything is
   written, and the response carries its `Content-Length`. A 406, and
   a value the negotiated format cannot carry, such as an infinite
   float in JSON, or whose encoder panics, are failures like any
   other: they go to the route's error handler (§scopes), whose answer
   replaces the response with its headers and cookies, so the default
   handler's answer is a 500 problem (the 406's own problem for a 406),
   never a 200 with a cut body or a connection closed without one. The
   error handler answers a render failure once; when its own answer
   fails the same way, the default handler answers for that failure,
   and the error handler is not called again. The top-level value
   renders alike in both formats: a nil `v` is the status without a
   body; a nil pointer, which GOB cannot encode and JSON would write as
   `null`, is an encoding failure in both; a nil slice or map is JSON
   `null` and an empty GOB value, which the generated client decodes as
   an empty slice or map. Below the top level the formats differ as
   their encoders do, and Volt does not walk the value to hide it: a
   nil pointer inside a slice, array or map is JSON `null` but a GOB
   encoding failure, so a 500 problem in GOB; a nil pointer in a struct
   field is JSON `null` and an absent GOB field, both decoding to nil;
   a pointer to a nil pointer is JSON `null` but GOB's zero value of
   the pointed type, since GOB flattens pointers. A value meant for
   both formats carries no nil pointer in a collection. A generated client decodes the body into a
   `*volt.ProblemError`, so a caller reads `errors.As(err, &pe)` and
   marks the fields `pe.Details` name.

### Generated client

Every routing package also generates a **client package**,
`client/volt_client.go` beside it, so a Go program — the desktop
application that shares the schema, a test, another service — calls
the routes through typed methods instead of hand-built requests.

1. The package is named `client` and imports only the standard
   library, the root `volt` package, the data packages the routes go
   through and, when a method takes an optional parameter (rules 3 and
   4), the data runtime `nao/rt` for its `rt.Null[T]`, never the runner
   `nao/run` and never the routing package, so a caller links no
   server code. The one exception is a package
   that routes its own tables (§query-routes): its client imports it for the
   row types, and so links it, under the package's name, or as `volt`
   and the name with an upper-case initial (`voltRt`) when the name is
   one the client imports for itself, `context`, `http`, `rt` or
   `time` (§imports); when that package is `main`, which Go
   cannot import, no client is generated.
2. `type Client struct { volt.Client }` and `New(base string)
   *Client`. The embedded runtime client carries the origin, the
   `http.Client`, the wire format (§formats) and `Header`, the default
   headers of every request it sends, through `Do`, `Raw` and `Stream`
   alike. `New` (`NewClient` beside the models, rule 6) sets `Header`
   to an empty `http.Header`, so `c.Header.Set("X-User", "1")` on a
   fresh client reaches every call that follows; a nil `Header` sends
   none. The headers the client writes itself are its own and never
   come from `Header`: `Accept`, the format's media type
   (`text/event-stream` on a stream); `Content-Type`, the format's when
   a body goes and absent otherwise; and a stream's `Last-Event-ID`,
   absent on its first connection. `Do` and `Raw` write no
   `Last-Event-ID`, so one in `Header` goes out on them as any other
   header does. `Stream` reads the client once, when it is called, so
   its reconnections, which run on a goroutine of their own, send the
   headers of that moment; a change to the client afterwards reaches
   the next call, not a stream already open. A non-2xx reply comes
   back as an `HTTPError`: a `*volt.ProblemError` carrying the Problem's
   status, message and details when the body decodes as one (§formats),
   else the status and the body text, so
   `errors.Is(err, volt.ErrNotFound)` holds for a 404.
3. **Every query route** (§query-routes) is one typed method, named as its
   helper would be (scope name prefixes and the function name, or
   `name:`), with the query's parameters in signature order and the
   query's result: `func (c *Client) APIUserGet(ctx, id int)
   (db.User, error)`, `APIUserDelete(ctx, id int) error`. Path
   parameters render through the same builders as `Path*` helpers,
   query-string parameters through `volt.FormatQuery`, a `time.Time`
   as RFC 3339 with its fractional seconds, so the instant arrives
   whole, a list as a
   repeated key, an optional parameter (§predicates) as the
   `rt.Null[T]` the query takes, left out of the query string when
   absent, `APIPostFeed(ctx, tag rt.Null[string]) ([]db.Post, error)`,
   a changeset as the body: `APIUserCreate(ctx, cs
   db.UserChangeset) (db.User, error)`, `APIUserUpdate(ctx, id int, cs
   db.UserChangeset) (db.User, error)`, sent as `cs.Params()`
   (§changesets). Each method sends its route's verb, the one the
   query fixes (§query-routes): GET for a read, POST for a create,
   PATCH for an update, DELETE for a delete. A changeset built from
   `<Model>New()` is the carrier of changes on the client side, so an
   update needs no row in hand:
   `APIUserUpdate(ctx, id, db.UserNew().EmailSet(e))` sets one column
   of row `id`.
   A method's Go parameters are named as the path parameters, the
   query's parameters or the query keys they carry, with `_` appended
   while the name is a Go keyword, a name the method uses (`ctx`, `c`,
   `cs`, `body`, `volt`, `http`, `context`, `rt`, `time`, `nil`,
   `append`, `any`, `error`), a qualifier the client imports a data
   package under (`db`), of the form of a generated local, or a name an
   earlier parameter of the method took: a select parameter `:db` is
   `db_`, and the keys `ctx` and `ctx_` are `ctx_` and `ctx__`. The
   changeset is always `cs`, a name no other parameter takes.
4. **Every named controller route** is one raw method with the
   route's path parameters, then every query-string key its loads bind
   (§route-loads), once each, in the order the loads first read it and
   typed and sent as a query route's parameter is (rule 3), in the one
   type every load binds it as, required when any load requires it, and
   named as rule 3 names parameters, so a route that loads through a
   `limit:` select sends its limit,
   `Mine(ctx, at rt.Null[time.Time], limit int)`, and, when the route
   binds a body (§request-bodies), the body last: a changeset `cs db.UserChangeset`
   sent as `cs.Params()`, or `body any` for a struct body, encoded in
   the client's format. It returns `*http.Response` for the caller to
   read and close: Volt does not know what a hand-written action
   writes.
5. Client method names and reverse-URL helper names share one
   namespace per package (§route-names-and-reverse-urls); a collision
   is an error naming what renames the later route: its `name:`; for
   resources, which take no `name:`, `singular:` on a member route
   (§settings); and always the name of an enclosing scope, which
   prefixes every name beneath it (§scopes) and is the only renaming a
   resources collection route or a dataset route has. The namespace
   also holds the names of the embedded runtime client: `Client`, its
   fields `Base`, `HTTP`, `Format` and `Header`, and its methods `Do`,
   `Raw` and `Stream`, which the generated methods call. A route that
   derives one of them is an error naming what renames it in the same
   way.
6. **Beside the models.** `volt gen -o DIR` (§projects-and-packages) writes the client
   as `DIR/volt_client.go` in the package `DIR` holds instead of as a
   subpackage, for a program that is the client — a desktop or browser
   front end that is `package main`. The row, changeset and params
   types are named bare, since the models written beside it declare them; the
   constructor is `NewClient(base string) *Client`, since `New` beside
   it may be the query layer's; nothing but the standard library, the
   root `volt` package and, for an optional parameter, `nao/rt` is
   imported.

### Event routes

An **event route** streams server-sent events to every connected
client, so what one user does — starts a computation, finishes it,
changes rows — reaches every open screen without polling.

```volt fragment
scope / [pipe: api] {
	get /events volt.Events
}
```

1. The handler `volt.Events` names the runtime's event stream; it is
   the only handler the runtime provides, an `http.Handler` of its own
   that streams for as long as the client listens rather than a
   function to a response value, and it takes `get`. The
   route otherwise behaves as any route: prefixes, pipelines, error
   handler, a helper and a client method named by the action
   (`PathEvents`, `client.Events`).
2. The `Controllers` manifest (§handler-references) gains `Events *volt.Broker`; the
   application publishes with `Events.Publish(name, data)`. The
   broker fans every event out to every open stream, numbers events in
   order, and keeps a replay buffer of the last `Replay` events (256
   unless set). An ID on the wire is `<epoch>-<n>`: the position and
   the broker's epoch, read from the clock in nanoseconds when the
   broker first numbers or serves an event and distinct among the
   brokers of one process, so the IDs of a process and of the one that
   replaced it differ unless both read the clock at the same
   nanosecond. A stream that sends back a `Last-Event-ID` of this
   broker resumes after it. Every other stream starts at the oldest
   event the buffer keeps: one opened without the header, one naming
   an ID of another broker, one from before the process restarted
   however far the new broker has counted, and one naming an ID in any
   other form, a bare position among them. An event delivered from the
   buffer carries the comment line `: replayed`, which `EventSource`
   ignores and `volt.Event` reads as `Replayed`, so a client tells what
   was published before it connected from what happens while it
   listens. A `volt.Event` keeps the broker's `Epoch` beside its `ID`,
   so a consumer tells a restart and deduplicates by the pair, and
   `WireID()` spells the two as the wire does. The client sends back
   the ID of the last event it delivered, so after a replay it resumes
   on the new broker's IDs. `Broker.Subscribe(after)`, from a
   position, and `Broker.SubscribeAfter(lastEventID)`, from a wire ID,
   are the same streams as a channel, for a program that reads events
   in process. Events are `text/event-stream` with a JSON payload; a
   subscriber that falls behind is dropped and reconnects.
3. The generated client's `Events(ctx) <-chan volt.Event` connects,
   delivers every event, reconnects with backoff on any drop, and
   closes the channel when ctx is done.
4. Event names and payloads are the application's contract, not the
   language's: a `table.changed` event carrying keys, after which the
   client refetches what it shows, keeps one code path for loading.

### Scopes

```ebnf
scope      = "scope", route path, [ settings ], "{", { scope item }, "}" ;
scope item = route | resources | dataset | scope | scope load ;
```

`scope` settings (the complete set):

| Setting | Value | Meaning |
|---|---|---|
| `pipe` | `pipeline` name | append the pipeline to the inherited chain |
| `name` | identifier | prefix for helper names beneath this scope |
| `error_handler` | function of this package, named bare (`Name`, §imports) | error spine for routes beneath, nearest wins |

1. Scopes nest arbitrarily. Prefixes concatenate; `pipe` chains
   **append** (ancestors outermost); `name` prefixes concatenate;
   `error_handler` is overridden by the nearest enclosing setting.
2. A route's middleware is the concatenation of its pipeline chain's
   plugs, composed once at generation time — never iterated per
   request.
3. The `error_handler` function is a Go reference held like a plug:
   it MUST exist in the package's compiled Go files with the volt
   runtime's ErrorHandler shape:
   `func(volt.Request, error) volt.Response`, a function from the
   request and the failure to a done response
   (§requests-and-responses). Routes without one use the runtime
   default, `volt.DefaultErrorHandler`, which a custom handler calls
   for the status, message and details the default would write from
   `volt.ProblemOf(err)` (§formats), since a refused write, a
   `*rt.Invalid`, carries no status of its own
   (§data-and-network-layers).

### Route names and reverse URLs

1. Every route derives a **helper name**, except a query route that
   writes (§query-routes) and the `create`, `update` and `delete`
   actions of resources (§the-action-table): the scope name prefixes
   followed by the `[name:]` setting (normalized to a Go name) when
   present, else the action name. Resources derive per §table-resolution.
2. Each named route yields one generated function
   `Path<Helper>(typed params..., opts ...volt.URLOption) string` that
   produces exactly the paths its route matches. Helper names form one
   namespace per package; a collision is an error. Helper output is
   always a clean path: string parameters are percent-escaped, and the
   values `.` and `..` escape entirely (`%2E` forms), since as literal
   segments they would change the path's shape under cleaning; a
   wildcard value keeps its `/` separators but drops its empty elements
   (`a//b` renders `a/b`, a leading `/` vanishes), the form ServeMux
   would redirect to.

### Route conflicts

1. A route's **shape** is its full path with every parameter replaced
   by `{}` and every wildcard by `{...}`; literals compare by spelling.
   (Braces cannot appear in a literal segment, §route-paths, so the markers
   cannot collide with any literal.)
2. Two routes **overlap** when some request matches both: their methods
   overlap (equal, either is `any`, or one is `get` and the other
   `head`, since ServeMux serves HEAD for a GET pattern, §routes) and
   their paths overlap (some request path satisfies both patterns; the
   root path `/` satisfies a wildcard at the root, whose rest is then
   empty). A route is **more specific** than another when every request
   it matches the other also matches, and not conversely; the methods
   of a `head` route are a subset of a `get` route's. Two overlapping
   routes where **neither is more specific** are ambiguous, an error
   naming both source positions: `get /a/:x` and `head /:y/b` both
   match HEAD `/a/b`.
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
scope / [pipe: api, error_handler: Errors] {
	get /            Home.Index [name: root]
	scope /admin [name: admin] {
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

1. `resources <table>` appears only inside a `scope` body and expands to
   the action routes of §the-action-table with the table name as the collection
   segment. The name MUST map to a Go identifier.
2. The declaration MUST resolve to a declared table (§table-resolution); there
   is no schemaless form. A miss — including a name that only matches
   a table's model name or differs only in case — is an error naming
   the correct spelling where one exists.

### The action table

| Action | Method | Path | Helper |
|---|---|---|---|
| `index` | GET | `/<table>` | plural |
| `new` | GET | `/<table>/new` | `New` + singular |
| `create` | POST | `/<table>` | none |
| `show` | GET | `/<table>/:<param>` | singular |
| `edit` | GET | `/<table>/:<param>/edit` | `Edit` + singular |
| `update` | PATCH | `/<table>/:<param>` | none |
| `delete` | DELETE | `/<table>/:<param>` | none |

Every action is one route. `update` is PATCH alone, the partial
update (§routes); the methods are those §query-routes fixes for the
CRUD function each API action maps to (§default-handlers). The key
segment is spelled as its type is (§route-paths): `:<param>(int)` for
an `int` key, a bare `:<param>` for a `string` one.

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
3. `singular` overrides the inflector of §table-resolution. It is
   required whenever singularization leaves the name unchanged and both
   `index` and `show` survive, since the collection and member helpers
   would otherwise collide.
4. The key parameter MUST NOT repeat a parameter of the enclosing
   scope (§route-paths): `Scope /blogs/:id { resources posts }` is an
   error, reported once at the declaration, naming `param:` as the
   fix (`[param: post_id]`). Under `[default]` the name is the
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
   the schema side, and when both apply `singular:` wins, since it
   renames the member helper and never the model (§names).
6. Plural and singular MUST differ where both helpers exist: a name
   whose singularization is the identity (`posty`, `data`, `series`)
   gives the collection and member helpers one name when both `index`
   and `show` survive, and that is an error naming `singular:` as the
   fix; with either action dropped by `only:` or `except:` the name
   stands.
   Neither a route `name:` nor a scope `name:` can resolve it — the
   former is not a resources setting (§settings-whitelists), the latter prefixes both
   sides equally.

```volt fragment
scope /api {
	resources db.users [only: (index, show, create)]
}
```

### Default handlers

```volt fragment
scope /api {
	resources db.users [default]
	resources db.tags  [default, except: (delete)]
}
```

1. `[default]` turns every surviving action into a **query route**
   (§query-routes) over the table's default CRUD function — `index` → `List`,
   `show` → `Get`, `create` → `Create`, `update` → `Update`, `delete`
   → `Delete` — so the handlers are generated and there is no
   controller to write. The routes, their paths and their helpers are
   those of §the-action-table and §table-resolution; only the handler side changes. Rails and
   Phoenix scaffold such a controller once and hand it over; Volt
   regenerates it forever, and an action taken back with `except:` is
   written by hand as any route, with the verb its method fixes, which
   is the action table's (§query-routes).
2. `default` implies `api`: `new` and `edit` are form pages, and a
   generated handler has nothing to return for them.
3. The table's CRUD is the data package's: an imported one for a
   qualified table (`db.users`), this package's own for a bare one
   (`users`; §query-routes, §imports), as for every query route. A bare table in a package that declares none is an
   error naming the qualified form.
4. The key parameter is named as the generated CRUD spells the
   primary-key column (`id` for `id`), because binding is by name
   (§query-routes); `param:` is an error with `default`. Type and
   routability follow §table-resolution unchanged.
5. An action whose CRUD function does not exist is an error naming the
   `except:` that drops it — `update` on a table whose every column is
   part of the key (there is nothing to set).
6. Naming: `index` and `show` keep their reverse-URL helpers and
   client methods (§table-resolution); `create`, `update` and `delete` have no
   reverse URL (writes never do, §query-routes) and a client method named
   `Create`/`Update`/`Delete` + singular, prefixed by the scope name
   like the form-page helpers `New`/`Edit` + singular would be.
   `update` is one PATCH route, the partial update: nothing the body
   leaves out is reset.
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
// db: Group series = ms_revenue + ms_usage
//     Select browse for series where year = :year
scope /ms [pipe: api] {
	dataset db.browse [strip: "ms_", except: (ms_usage)]
	get /usage Ms.Usage
}
```

### Dataset declaration

1. `dataset` appears only inside a `scope` body and names a select: of
   an imported data package when qualified (`db.browse`), the
   qualifier marking the import used (§imports); of this package when
   bare (`browse`; §query-routes, §imports). A name
   that is not a select of that package is an error, with a
   case-insensitive hint.
2. For every member of the select's target (§selects-over-groups), in member order,
   the dataset yields one `get` **query route** (§query-routes), the
   verb a select function fixes, whose path is
   the scope prefixes followed by one segment — the member's table
   name, with the `strip:` prefix removed — and whose handler is the
   member's select function (`<Model><SelectName>`). Every parameter of
   the select binds as §query-routes binds it: from a scope prefix
   parameter of the same name, else from the query string, a list
   parameter as a repeated key, and an optional one (§predicates) from
   the query string alone, absent when the key is missing; a prefix
   parameter the select does not take is an error, and so is one that
   names an optional parameter, since a path segment is always
   present, and the error names the fix: the dataset in a scope
   without that prefix parameter, or a second select whose parameter
   is required. A select with an optional list parameter, which no
   route carries (§query-routes), makes the dataset an error; each
   is reported once for the dataset, not once per member. The helper
   and client method names are the scope name prefixes followed by
   the function name (§query-routes).
3. Expanded routes obey every rule of routes: conflicts (§route-conflicts), one
   helper namespace (§route-names-and-reverse-urls, §generated-client). To hand-write one member's route,
   take it out with `except:` and declare it beside the dataset, with
   `get`.

### Dataset settings

| Setting | Value | Meaning |
|---|---|---|
| `strip` | string | prefix removed from each member's table name to form its segment |
| `only` | table list `(ms_revenue, ms_usage)` | keep only these members |
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
| `scope` | `pipe`, `name`, `error_handler` |
| route | `name`, `timeout`, `body`, `only` |
| resources | `api`, `only`, `except`, `param`, `singular`, `default` |
| dataset | `strip`, `only`, `except` |
| Select | `order`, `group`, `limit` |
| `migration` | `irreversible`, `no_transaction` |

The identifier-list value form `(a, b, c)` (production in Collected
grammar (Part II)) is valid only as the value of `only:`, `except:` and
a select's `group:`: an action list on resources, a table list on a
dataset, a column list on a route (§request-bodies), the grouping
columns of a select (§selects-over-groups). A select's
`order:` takes its order list (§selects-over-groups). The query-call
value form `pkg.Query(a)` is valid only as the value of a route's
`body:`. The parameter value form `:name` (§predicates) is valid only
as the value of a select's `limit:`, which takes it or a positive
integer (§selects-over-groups); a parameter as the value of any other
setting is an error.

## Generation contract (informative)

The normative output contract is the golden corpus under
`gen/router/testdata/` and the proof suite under `itest/`. In prose:
one `<Controller>Controller` interface per controller and a
`Controllers` struct (§handler-references), with the `DB run.DB` field the
shim performs effects on (§query-routes); `NewRouter(Controllers) http.Handler`
registering every route onto a `http.ServeMux` with its pipeline chain
composed statically and its typed shim parsing parameters per §route-paths
— the shim is a loop that performs a response's continuations and
writes the done response (§requests-and-responses), and for a query
route it is the whole handler: it binds every parameter per
§query-routes and returns the continuation that renders per §formats;
`Path*` helpers per §route-names-and-reverse-urls; a `Table []volt.Route` mirroring the
expanded route list in declaration order, with `Query` set on query
routes; and the client package of §generated-client. For a data package that
declares migrations, `volt_migrations.go` carries them in statement form
(§migration-tooling). Every generated file is named `volt_<part>`
(`volt_models.go`, `volt_queries.go`, `volt_dyn.go`, `volt_validate.go`,
`volt_selects.go`, `volt_migrations.go` and `volt_schema.sql` for a data
package; `volt_handlers.go`, `volt_router.go`, `volt_paths.go`,
`volt_routes.go` and `volt_client.go` for routes), carries the one header
`// Code generated by volt gen from package <path>. DO NOT EDIT.`
(`--` in place of `//` in the SQL file), and is gofmt-stable. `volt gen`
removes a file left under an earlier name of a data file (`nao_models.go`,
`nao_queries.go`, `nao_dyn.go`, `nao_validate.go`, `nao_selects.go`,
`nao_schema.sql`) when it carries the generated-code marker, and leaves
one without it. By the same marker it removes a generated file it no
longer writes: `volt_selects.go` once the package declares no select,
`volt_migrations.go` once it declares no migration, and the route files
once it declares no scope (§data-and-network-layers).

---

## Groups

A **`group`** names a set of tables so the same code can be generated for
every member (§selects-over-groups). It is the one set construct of the
language: groups overlap freely, a table belonging to any number of
them.

```ebnf
group decl = "group", name,
             ( "{", { newline }, [ group members ], "}"
             | "=", group expr ), newline ;
group members = name, { newline, { newline }, name }, { newline } ;
group expr    = group term, { ( "+" | "\" ), group term } ;
group term    = name
              | "(", name, { ",", name }, ")" ;
```

```volt fragment
group series {
  ms_revenue
  ms_usage
  ks_seats
}

group wide   = series + ks_costs \ ms_usage
group public = Commerce \ (audit_log, api_keys)
```

1. Tables and groups share one namespace per package
   (§namespaces-and-uniqueness): a group MUST NOT take the name of a
   table, and redeclaring a group is an error.
2. A name in a `group term` resolves, case-sensitively, to a table or a
   group of the same package, tables first; the same order resolves a
   select's target (§selects-over-groups). An unknown name is an error;
   one that differs only in case from a table or group names the right
   spelling, and a table's model name names the table.
3. The block form is the expression form with every member joined by
   `+`. Evaluation is left to right: `+` adds a term's member set
   (a table adds itself), `\` — set difference — removes it. A
   parenthesized term is the union of its names, applied one name at a
   time in the order written. Removing a table that is not currently a
   member, or adding one already present, is an error — the algebra
   must say something true. `-` is not a group operator: writing it is
   an error naming `\`.
4. `group` references must be acyclic (error otherwise), and the
   resulting set must be non-empty.
5. Member order is the order of the last addition: a table removed and
   added again moves to the end. Generation (§selects-over-groups) is
   deterministic in it.

## Predicates

A **`pred`** names a boolean expression over the columns of an as-yet
unnamed table. The expression language is deliberately **closed** —
it is not SQL and never grows toward it; anything it cannot say
belongs in the dynamic layer (§generated-query-surface).

```ebnf
pred decl    = "pred", name, "{", { newline }, pred expr, { newline }, "}", newline ;
pred expr    = pred and, { "or", pred and } ;
pred and     = pred unary, { "and", pred unary } ;
pred unary   = { "not" }, pred primary ;
pred primary = "(", pred expr, ")"
             | comparison | membership | pattern | null test | exists term
             | name - pred keyword ;       (* reference to a Pred *)
comparison   = operand, comp op, operand ;
comp op      = "=" | "!=" | "<" | "<=" | ">" | ">=" ;
membership   = column ref, "in", ( "(", literal, { ",", literal }, ")" | param ) ;
pattern      = column ref, "like", ( string | param ) ;
null test    = column ref, "is", [ "not" ], "null" ;
exists term  = "exists", name - pred keyword, "(", pred expr, ")" ;
operand      = column ref | param | literal ;
column ref   = [ name - pred keyword, "." ], name - pred keyword ;
param        = ":", name, [ "?" ] ;        (* no space after ':' or before '?' *)
literal      = [ "-" ], number | string | boolean ;
pred keyword = "and" | "or" | "not" | "in" | "like" | "is" | "null" | "exists" ;
```

```volt fragment
pred current { org = :org and year = :year }
pred recent  { year >= :since }
pred fresh   { current and recent }
pred chosen  { org in :orgs }
pred before  { published_at < :at? or (published_at = :at? and id < :id?) }
pred tagged  { exists tags (tag = :tag?) }
```

1. `and`, `or`, `not`, `in`, `like`, `is`, `null` and `exists` are
   keywords inside a predicate expression, lowercase (§keywords): a
   column or a relation of one of these names, in any case, cannot be
   referenced in a predicate. `true` and
   `false`, in any case, are the boolean literals where an operand
   stands, so a column of either name is referenced only before `in`,
   `like` or `is`, and a `pred` of either name only as a bare reference.
   Outside predicates they are ordinary identifiers.
2. A bare name in primary position references a `pred` of the same
   package; references must exist and be acyclic. `pred` names are unique
   in the package; redeclaring one is an error.
3. A `pred` is typed **at each use site** (§selects-over-groups), where a target binds
   its column names, so a fault a `pred` has over one select and not
   over another is the select's: it is reported where that select
   names the `pred`, naming the select, the `pred`s it went through and
   where in them the fault stands. The rules:
   - a comparison MUST name a column on at least one side; a parameter
     or literal on each side is an error;
   - a `column ref` must resolve in the target's environment
     (§selects-over-groups): a bare name is a column of the target, a
     qualified one, `metric.name`, a column of a relation the use site
     joins, typed as its own table types it;
   - `=` and `!=` require both operands to share a type class;
   - a comparison of two columns requires one Go type, nullability
     aside: an `int` column does not compare with a `float64` column,
     nor a column of a declared type with a column of any other type;
   - `<` `<=` `>` `>=` require numeric or date/time operands — text is
     **not** orderable here (`text_column > 1` and
     `text_column > "a"` are both errors; order text in SQL, §selects-over-groups);
   - `like` requires a text column and a text pattern;
   - `in` items must all match the column's type;
   - a numeric literal MUST be representable in the column's storage
     type (§columns): against `int` it is an integer literal that fits
     int64, against `float64` a finite value; `9223372036854775808`,
     `1e400`, and `1.5` or `1e2` against `int` are errors. The generated
     method binds the literal in canonical Go form, so `-010`, `007`
     and `1E2` bind `-10`, `7` and `100.0`, and an integer beyond 32
     bits as an `int64`, `int64(3000000000)`, which compiles where an
     `int` is 32 bits;
   - `in :name` makes `:name` a **list parameter**: its Go type is a
     slice of the column's Go type without `rt.Null` (`[]string` for
     both `string` and `string [null]`, `[]int`, `[]time.Time`); a
     `time.Time` item travels in the JSON array, the list's wire form
     (§selects-over-groups), as the text the column holds, its stored
     form (§go-types). Membership in an empty list is false, so
     `id in :ids` with no ids matches no row;
   - `[]byte` columns may not appear in predicates;
   - a `param` adopts the type of the expression position it appears
     in, the column's Go type without `rt.Null` for a nullable column
     (NULL is tested with `is null`); one param name MUST resolve to one
     type across the whole use site (a list and a scalar use of the
     same name disagree), or the later use is an error naming both
     types. A parameter name MUST be an ASCII Go identifier that is
     neither a Go keyword, nor `_`, the blank identifier, which no
     generated expression can read, nor a name the generated signatures use
     (`ctx`, `rt`, `w`, `r`, `opts`, `volt`, `cs`, `c`, `body`), nor `nil`,
     `append`, `any` or `true`, the predeclared names a generated
     function uses, and
     MUST NOT have the form of the generated locals and helpers,
     `volt`, an upper-case letter and no underscore
     (§generated-query-surface); the error names the one rule a name
     breaks. `q`, the usual search term, is a parameter like any other:
     `title like :q?`.
4. `[]byte` (rule 3) is the one column type predicates refuse: every
   other column, a column of a declared type among them, compares
   under rule 5.
5. Type classes follow the column types (§columns): numeric = `int`
   and `float64`; text = `string`; date/time = `time.Time`; `bool`
   agrees with `bool` only. A `time.Time` column takes no literal: the
   literal grammar has no date/time literal, and a string is text. A column of a declared type (§types)
   compares with `=`, `!=`, `in` and `is null` only: a parameter in
   that position has the declared Go type, a literal has the class
   of the storage type, and a declared type stored as `time.Time` or
   `[]byte` takes no literal. A list parameter of a declared type is
   bound as one JSON array of its values, by kind for a kinded type
   and through the type's driver value otherwise (§types), so a
   declared type stored as `time.Time` travels as the stored form of
   its driver value (§go-types). A declared type stored as `[]byte`
   cannot take one: JSON carries
   no blob, and the text its bytes would travel as never equals the
   stored value.
6. **Optional parameters.** `:name?`, the `?` directly after the
   name, makes a parameter **optional**: its Go type is `rt.Null[T]`,
   `T` the type rule 3 gives the parameter, so an optional list
   parameter is `rt.Null[[]T]` (`rt.Null[[]string]`), and the generated
   function takes it present, `rt.Some(v)`, or absent, the zero value.
   - A parameter is optional at every use in one select, the `pred`s it
     uses included, or at none: `:at?` beside `:at` is an error naming
     both uses. An optional parameter stands in a predicate term only:
     in a projection's arithmetic or as the value of `limit:`
     (§selects-over-groups) it is an error.
   - **The vanishing rule.** A *term* is a comparison, a membership, a
     pattern or a null test. A term reads the parameters it names, a
     null test none, so a null test is never taken out, and a term that
     reads an absent optional parameter is **taken out** of the
     predicate, as if it were not written: it is the identity of the
     connective it stands in, true under `and` and false under `or`, so
     what stands beside it is filtered as written. An `and`, an `or`, a
     `not` or an exists term (rule 7) all of whose terms are taken out
     is taken out itself, the same way, and a `where` all of whose
     terms are taken out keeps every row. So `author_id = :by? or exists tags
     (tag = :tag?)` keeps the posts of `:by` when `:tag` is absent,
     those tagged `:tag` when `:by` is, and every post when both are;
     `exists comments (author_id = :by? and body like :text?)` keeps
     the posts `:by` commented on when `:text` is absent, and every post
     when both are; and `published_at < :at? or (published_at = :at?
     and id < :id?)` keeps every row when both are absent, which makes
     the first page of a keyset over `order: (published_at desc, id
     desc)` the same select as every later page. A term that reads no
     absent parameter means what it says, so `published_at < :at? or
     published_at is null` keeps the rows never published when `:at`
     is absent. A `not` negates what of its operand is left and never
     a term taken out, which is not there to negate: `not exists tags
     (tag = :tag?)` keeps the posts not tagged `:tag`, and every post
     when `:tag` is absent, and `not (views >= :min? or author_id =
     :by?)` keeps the posts with fewer than `:min` views when `:by` is
     absent.
   - **Absent is what binds NULL.** A guard (below) reads the value the
     parameter binds, so the parameter is absent whenever that value
     binds as NULL: the zero `rt.Null[T]`, and a present value whose
     driver value is NULL too, a `float64` NaN, which SQLite binds as
     NULL, or a value of a declared type whose `Value` answers nil
     (§types). A query route takes no NaN for a `float64`
     (§query-routes). A nullable column's field has the parameter's
     own type, `rt.Null[T]`, so a value read from such a column passes
     as an optional parameter, and its NULL is absent:
     `OrderPlacedBy(o.PlacedAt)`, over `placed_at < :before?`, filters
     nothing for an order never placed, where `placed_at < :before`
     would match no row. Pass a value that may be NULL as an optional
     parameter only where its NULL should mean no filter; NULL itself
     is tested with `is null` (rule 3).
   - A keyset pages soundly only over rows whose `order:` columns are
     never NULL. Its cursor is the last row's values of those columns,
     and a comparison never matches NULL, so no page after the first
     reaches a row whose column is NULL; worse, a cursor taken from a
     row whose column is NULL is absent (above), so the terms that read
     it are taken out, and the page after that row reads by the key
     alone, again reading rows earlier pages read.
     The keyset above therefore takes `published_at` to be declared
     without `[null]`; over a nullable column, the `where` keeps the NULLs out,
     `published_at is not null and (published_at < :at? or …)`, or the
     keyset pages over key columns alone.
   - An optional list parameter that is present and empty keeps rule
     3's meaning: membership in an empty list is false, so
     `tag in :tags?` keeps every row for an absent list and none for an
     empty one.
   - The statement stays one constant text whatever is present: a term
     that reads optional parameters renders behind one guard per
     optional parameter it reads, the guards in first-appearance order,
     `(? IS NULL OR <term>)` under `and` and `(? IS NOT NULL AND
     <term>)` under `or`; an `or` under `and`, an `and` under `or`, a
     `not` and an exists term, each of which every term of can be taken
     out, render behind the condition that all of them are, `((? IS NULL AND ? IS
     NULL) OR <it>)` under `and` and `(NOT (? IS NULL AND ? IS NULL) AND
     <it>)` under `or`, each parameter once. The predicate of a `where`,
     the one inside an exists term and the operand of a `not` stand
     under `and`: at the top of any of them a term renders behind its
     `and` guard and an `or` behind the `and` form of its condition, so
     it is true when every term of it is taken out, and the `not` over
     an operand so taken out renders behind its own condition, which
     takes the `not` out with it. Every placeholder of
     the parameter, a guard's and the term's own, binds the parameter's
     `rt.Null`, which binds NULL when absent (rule 8). An optional list parameter binds
     as `rt.JSONListOptional(tags)`: NULL when absent, the JSON array of
     rule 3 when present. The one text costs an index: SQLite plans the
     statement once for whatever it binds, so a guarded term finds no
     row through an index, even when the parameter is present. `"site"
     = ?` searches an index on `site`; `(? IS NULL OR "site" = ?)`
     reads every row, and so does an exists term's subquery for the
     guards inside it. Where a filter should
     search an index, write it with a required parameter, in a select
     of its own.
7. **`exists`.** `exists rel (x)` is true for a row when at least one
   row that the relation `rel` leads to from it matches `x`.
   - `rel` is a relation of the select's `for` table, named as `join`
     names one (§relationships-ref, §selects-over-groups): a forward
     relation, which leads to the one row the foreign key refers to, or
     a reverse relation, the rows that refer to this one. It MUST NOT
     be ambiguous, and the target MUST be a table, since a group has no
     relations. A name that is no relation of the table is an error
     listing its relations.
   - `x` is a predicate over the related table's columns, named bare:
     inside `exists`, a bare name is a column of the relation's table,
     typed as its own table types it (rules 3 to 5), and a qualified
     name is an error. It sees no column of the outer row: the
     relation's key is the one link between the two. A `pred` it
     references expands there, over the same table. The `where` of a
     select and a `pred` both take the term; a `pred` is typed at each
     use site, so one that holds an exists term serves a table that has
     the relation.
   - `exists` does not nest: an exists term inside another one, written
     or reached through a `pred` reference, is an error. The depth is
     one, as it is for `with` (§selects-over-groups).
   - `exists` reaches what `join` reaches but filters without joining.
     A join of a reverse relation repeats the row of the `for` table
     once per related row, so filtering through it multiplies the rows
     every aggregate reads: in `select feed (id, count(comments.id) as
     n) for posts join comments join tags where tags.tag in :tags
     [group: (id)]` a post with two of the tags counts each comment
     twice. `exists tags (tag in :tags)` keeps the post once and its
     comments once each; an exists term repeats no row, so `limit:`
     needs no key of the relation's table for it (§selects-over-groups).
   - The term renders as a correlated subquery (rule 8): `EXISTS
     (SELECT 1 FROM "post_tags" AS "tags" WHERE "tags"."post_id" =
     "posts"."id" AND ("tags"."tag" = ?))`.
8. **Rendering.** There is one SQL renderer for the generated query
   surface, the runtime's (`rt.SelectRender` and its siblings): the
   select functions, the CRUD functions, the changeset writes and the
   dynamic layer are its output. The dynamic layer calls it at run
   time, the generator calls it at generation time over the same tree
   and writes the result into the statement constants, so the same
   predicate spells the same bytes on both paths. Its spellings are
   fixed and it produces no other:
   - an identifier is always double-quoted (`"email"`), a `"` inside
     it doubled; a qualified column quotes both parts,
     `"metric"."name"`;
   - a comparison is `"col" <op> ?` or `"col" <op> "other"` with
     `<op>` one of `=`, `!=`, `<`, `<=`, `>`, `>=`; `!=` is the one
     spelling of inequality, `<>` is never produced and the dynamic
     layer has no operator for it;
   - membership in values is `"col" IN (?, ?)`, `NOT IN` likewise; an
     empty list is the constant `1 = 0` (`1 = 1` negated); a list
     parameter is `"col" IN (SELECT value FROM json_each(?))`, bound
     as one JSON array (`rt.JSONList`); the dynamic layer's `InList`
     over a list any item of which binds as bytes, a plain `[]byte`
     or a value whose driver value is `[]byte`, is membership in
     values instead, each item bound as an `In` item is, since JSON
     carries no blob (rule 5);
   - a pattern is `"col" LIKE ?`, `"col" NOT LIKE ?`; a null test
     `"col" IS NULL`, `"col" IS NOT NULL`;
   - `and` and `or` of two or more operands are parenthesized,
     `(a AND b AND c)`; one operand is itself; `not` is `NOT (x)`, and
     when x brings its own parentheses (an `and`/`or` group, a raw
     fragment) they serve: `NOT ("a" = ? OR "b" = ?)`;
   - an exists term (rule 7) is `EXISTS (SELECT 1 FROM <table> AS
     "<relation>" WHERE <pairs> AND (x))`, `<table>` the relation's
     table, `<pairs>` the relation's key as a join spells it,
     `"<relation>"."<column>" = "<target>"."<column>"` per key column
     joined by ` AND `, the target's column qualified by the target
     table's name whether or not the select joins, and every column of
     x qualified by the relation's name; the parentheses of x serve
     when it brings its own, as under `not`;
   - a term that reads optional parameters (rule 6) is `(? IS NULL OR
     <term>)` under `and` and `(? IS NOT NULL AND <term>)` under `or`,
     one guard per optional parameter it reads, in first-appearance
     order: `(? IS NULL OR "published_at" < ?)`; an `or` under `and`,
     an `and` under `or`, a `not` and an exists term every term of
     which reads an optional parameter are `(<all absent> OR <it>)` and
     `(NOT <all absent> AND <it>)`, `<all absent>` the parameters' `? IS NULL`
     joined by ` AND `, each once: `((? IS NULL AND ? IS NULL) OR
     EXISTS (SELECT 1 FROM …))`; the predicate of a `where`, the one
     inside an exists term and the operand of a `not` stand under
     `and`, so an `or` at the top of any of them is `(<all absent> OR
     <it>)`: `((? IS NULL AND ? IS NULL) OR
     ((? IS NOT NULL AND "published_at" < ?) OR …))`; each guard binds
     its parameter's value, as every placeholder of the term does;
   - a value is always an argument behind a positional `?`, in
     left-to-right order of the text: a literal written in a Volt
     predicate is bound by the generated method like a parameter, the
     statement text carries no literal (the constants `1 = 0`, `1 = 1`
     and `-1` of `LIMIT -1 OFFSET ?` are structure, not values);
   - a projected item (§selects-over-groups) is the column as above,
     `count(*)` for a bare `count()`, `count("col")`, `sum("col")`,
     `avg("col")`, `min("col")` or `max("col")`, or an arithmetic
     expression with every operation parenthesized and every number
     and parameter an argument, `(("ts" / ?) * ?)`; a derived item is
     followed by ` AS "alias"`, and the select list's arguments precede
     the WHERE clause's;
   - a grouping is `GROUP BY` after the WHERE clause, naming a column
     as above and a derived item by its quoted alias, which SQLite
     resolves, so no expression is spelled twice; an ordering names an
     alias the same way;
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
     <predicate>][ GROUP BY <items>][ ORDER BY <terms>][ LIMIT ?[ OFFSET
     ?]]`, the column list and the table the pre-rendered constants of
     the model; an offset without a limit is `LIMIT -1 OFFSET ?`;
   - a select with joins (§selects-over-groups) is the same with
     `<table>` followed by one ` INNER JOIN <other> AS <relation> ON
     <pairs>` per join, `LEFT JOIN` for an optional relation, in
     declaration order; `<pairs>` is `"<relation>"."<column>" =
     "<table>"."<column>"` per key column, joined by ` AND `, and every
     column of the statement is qualified, the table's own by the
     table's name;
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
   The DDL (§mapping-to-sqlite-ddl), the migrations' statements
   (§migrations) and the migration ledger's (§migration-ledger) are
   not its output and follow their own spelling. A test renders a corpus of predicates once through the typed
   handles and once from Volt source and compares the bytes; the
   generator goldens and the integration tests prepare the constants
   on a real SQLite. An exists term and an optional parameter have no
   typed handle: a Go caller of the dynamic layer leaves out a
   predicate it has no value for, since a zero predicate vanishes, and
   writes a subquery as `rt.Raw`. Their spellings are pinned by a test
   of the renderer, by the generator goldens and by the integration
   tests, which run them on SQLite.

## Selects over groups

A **Select** declares a query once and generates it for every member
of a target: a group (§groups), or a single table treated as a
one-member group.

```ebnf
select decl = "select", name, [ projection ], "for", name, { join },
              [ with ], [ "where", pred expr ], [ settings ], newline ;
join        = "join", name ;
with        = "with", name ;

projection  = "(", proj item, { ",", proj item }, ")"
            | "(", "*", "\", column term, { "\", column term }, ")" ;
proj item   = column name
            | ( aggregate | arith ), "as", own column ;
aggregate   = "count", "(", [ column name ], ")"
            | ( "sum" | "avg" | "min" | "max" ), "(", column name, ")" ;
arith       = arith term, { ( "+" | "-" ), arith term } ;
arith term  = arith factor, { ( "*" | "/" ), arith factor } ;
arith factor = column name | number | param | "(", arith, ")" ;
column term = own column
            | "(", own column, { ",", own column }, ")" ;
column name = [ name, "." ], own column ;
own column  = name ;
order list  = "(", order term, { ",", order term }, ")" ;   (* the value of order:, a setting value (§settings-lists) *)
order term  = column name, ( "asc" | "desc" ) ;
```

```volt fragment
select rows    for series where current [order: (year desc, id asc)]
select stale   for ms_usage where not recent
select all     for series
select summary (id, org, year)    for series where current
select public  (* \ password_hash) for accounts
select brief   (* \ (password_hash, bio, avatar)) for accounts
select recent  (id, ts, value, metric.name, metric.unit) for points join metric where ts >= :since [order: (ts desc)]
select hourly  (metric_id, ts / 3600 * 3600 as hour, avg(value) as mean, count() as n) for points where ts >= :since [group: (metric_id, hour), order: (hour asc)]
select totals  (count() as n, max(value) as top) for points
select board   for metrics with points where unit = :unit [order: (name asc)]
select latest  for points where ts >= :since [order: (ts desc, id desc), limit: :limit]
select peaks   (metric_id, max(value) as peak) for points [group: (metric_id), order: (peak desc, metric_id asc), limit: 10]
select feed    for posts where exists tags (tag = :tag?) and (published_at < :at? or (published_at = :at? and id < :id?)) [order: (published_at desc, id desc), limit: :limit]
```

The clauses read in SQL order — name, columns, source, filter — and
the optional projection narrows the emitted columns.

1. The select name is an identifier; per target-member it mints
   the function `<Model><SelectName>` (§names). A select name that is a
   generated suffix (`Get`, `GetMany`, `List`, `Create`, `Update`,
   `Delete`, `Count`, `Exists`, `Query`, `DeleteWhere`, `UpdateWhere`,
   `Limit`, `Offset`, `OrderBy`, `Changeset`, `Column`, `Params`,
   `New`) is an error, and so is a minted function that takes the name
   of any other generated name of the package: a CRUD, `GetBy<Field>`
   or dynamic function of any table, a column handle, another select's
   minted function, or any name of §changesets. Its statement constant
   is named from the function, so a minted `<Model>Table` or `<Model>Columns`,
   which would take a table's statement constants, is an error too.
2. `for` names a table or a group of the package, resolved as a group
   term is (§groups): case-sensitively, tables first, with the same
   aids (§groups). Since a group cannot take a table's name, at most
   one declaration matches.
3. `where` takes a full §predicates expression — named Preds, inline
   comparisons, exists terms over the target's relations, and any
   composition of them. Omitted = all rows.
4. **The agreement rule.** Every column referenced by the `where`
   expression (`pred` references expanded) and by `order:` MUST exist in
   **every** member, and all members MUST agree on its type (§predicates);
   a column inside an exists term is its relation's table's instead
   (§predicates), and the target of one is a table.
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
6. Generation contract (with §mapping-to-go): per member, a function
   of `volt_selects.go` returning the query as a value
   (§generated-query-surface) —
   `func <Model><SelectName>(<params>) rt.Query[<Row>]`
   — parameters in first-appearance order over the expanded
   expression, each typed by §predicates, an optional one as
   `rt.Null[T]` (`func PostFeed(tag rt.Null[string], at
   rt.Null[time.Time], id rt.Null[int], limit int)
   rt.Query[Post]`), and the `limit:` parameter
   last (rule 12); `<Row>` is the member's row
   type (the model itself when there is no projection); the
   emitted SQL is the runtime renderer's output (§predicates,
   rendering), `SELECT <projected columns> FROM <table> [WHERE …] [ORDER BY
   …] [LIMIT ?]` with positional `?` placeholders, the function binding its
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
   - **Explicit list** `(a, b, c)` — at least one item, no
     duplicates; a derived item (rule 9) is listed by its alias. Every listed column MUST exist in every member with
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
     **struct derivative** named `<Model><SelectName>Row`
     (`User` + `public` → `UserPublicRow`, the query function being
     `UserPublic`): every kept field is copied
     verbatim from the model — Go name, Go type, assembled struct tag
     (§struct-tags, `tag:` passthroughs included), doc comment, so the
     derivative behaves exactly like the model minus the removed
     fields.
   - Columns referenced by `where` or `order:` need not be projected.
   - Minted row-type names (the shared struct, and each derivative)
     live in the generated package scope and MUST NOT collide with any
     generated package-level name. The checker reports collisions with
     every name the generated code mints for any table of the package
     (the model, `<Model>Changeset`, `<Model>Column`, `<Model>Params`,
     `<Model>New`, the column handles and the dynamic-layer wrappers
     `<Model>Limit`, `<Model>Offset`, `<Model>Distinct`,
     `<Model>OrderBy`, `<Model>After`, `<Model>Set`), the package's bare
     declared types (§types), and another select's minted row type; the select names `Changeset`,
     `Column`, `Params` and `New` are rejected outright (rule 1). The Go
     compiler backstops the rest.
8. **Joins.** `join` names a relation of the `for` table
   (§relationships-ref), each at most once and none ambiguous; the
   target MUST be a table, since a group has no relations. A required relation joins
   inner and an optional one left. The joined table's columns are
   reached **qualified** by the relation's name, `metric.name`, in
   the projection, in `where` and in `order:`, and a bare name stays
   the `for` table's own; a qualifier that names no joined relation,
   or a column the relation's table lacks, is an error. A select with
   a join MUST carry an explicit projection, whose row type is the
   shared struct the select name mints (rule 7): a qualified column's
   field is `<Relation><Column>`, typed as the column is in its table
   but `rt.Null[T]` under a left join, with the wire name
   `<relation>_<column>`; two projected columns with one Go field name
   are an error naming both. Under `where` and `order:` a qualified
   column obeys the rules of its own table's column (§predicates). The
   statement is the renderer's join spelling (§predicates, rendering):
   `SELECT "points"."id", "points"."ts", "metric"."name" FROM "points"
   INNER JOIN "metrics" AS "metric" ON "metric"."id" =
   "points"."metric_id" WHERE "points"."ts" >= ?`, `LEFT JOIN` for an
   optional relation, a composite key `AND`ing its pairs, every column
   qualified once a join is present. The relation name is the SQL
   alias, which is why a relation named as its table is ambiguous
   (§relationships-ref). A filter on a relation needs no join: an
   exists term (§predicates) tests it and adds no row. The two differ
   for a reverse relation, whose join repeats each row once per
   related row, which inflates every aggregate the select computes; a
   forward relation leads to one row at most, so a filter through its
   join and the exists term over it keep the same rows, and a join is
   what projects or orders by the relation's columns.
9. **Aggregates and arithmetic.** A projected item is a column, an
   aggregate or an arithmetic expression; the latter two are
   **derived** and carry an alias, `as name`, a lower-case identifier
   that maps to a Go name and names no column of the target or of a
   joined table, since SQLite resolves a `GROUP BY` name against the
   tables before the aliases; the alias names the item in `group:`, in
   `order:` and in the row type, where its field is the alias's Go
   name with the alias as the wire name.
   `count()` counts the rows and `count(c)` the rows where `c` is not
   NULL, both `int`; `sum` and `avg` take a numeric column, `avg` is
   `float64` and `sum` the column's type; `min` and `max` take a
   numeric, date/time or text column and keep its type. An arithmetic
   expression combines numeric columns, number literals and parameters
   with `+`, `-`, `*` and `/`, parenthesized freely, under the type
   rules of §predicates: its columns share one Go type, which is the
   expression's and its parameters', a literal is representable in it,
   and `/` on `int` is SQLite's integer division; a lone column under
   an alias renames it, whatever its type. A derived item is
   `rt.Null[T]` where it may be NULL: an aggregate other than `count`
   over a nullable column or, without `group:`, over no rows; an
   expression over a nullable column; and a column reached through a
   left join counts as nullable here, as it does projected bare. Qualified columns reach joined
   relations (rule 8) in aggregates and expressions alike, and a
   derived item's Go field obeys the collision rule of rule 8.
10. **`group:` and one-row selects.** `group:` takes an identifier list
    naming the non-aggregate items, a column item by its name and a
    derived one by its alias, every one of them and nothing else, each
    once; a joined column goes in through an alias. A select that
    aggregates without `group:` yields exactly one row, so it is
    generated as an `rt.Lookup` of the row type rather than an
    `rt.Query`, and `order:` and `limit:` on it are errors. `where` sees columns,
    never aliases, and there is no `having`: a filter over an aggregate
    is the one escape, a statement written in Go against the runtime
    (§generated-query-surface).
11. **`with`: nested results.** `with` names a reverse relation of the
    `for` table (§relationships-ref), which MUST be a table, over one
    key column that is never NULL and travels as a list parameter
    does (§predicates: not `[]byte`; a `time.Time` key travels as its
    stored form, §go-types); the relation is
    not ambiguous, the select does not aggregate, and when it
    projects, the key column is a bare item of the projection. The row type is `<Row>With<Relation>`, a struct
    embedding the parent's row type `<Row>` (rule 7) with a field named
    after the relation that holds the children, the child table's full
    rows: `MetricWithPoints { Metric; Points []Point }`; the name joins
    the minted names of rule 7, selects that nest one relation under
    one parent row type share it, and a parent field of the child
    field's name is an error. The function yields `rt.With[Parent, Child, Key,
    Row]`, an effect of `[]Row` (§generated-query-surface), performed
    as two statements in one transaction, so both read one
    snapshot: the parent statement as any select's, then the child
    table's rows `WHERE "<key>" IN (SELECT value FROM json_each(?))`
    over the parent keys, ordered by the child's primary key, grouped
    in memory by key and nested in parent order. No child statement
    runs for no parents, a parent without children carries an empty
    slice, and a NULL key never matches. One level, no projection of
    the children, and no `with` across a junction table.
12. **`limit:`.** `limit:` keeps the first rows of the select in the
    order `order:` sets. Its value is a positive integer literal,
    `limit: 10`, or a parameter, `limit: :limit`, the one setting value
    that may be a parameter (§settings-whitelists). The literal is
    decimal digits whose value is at least 1 and fits `int64`, bound
    in canonical form as a predicate's literal is (§predicates), so
    `limit: 007` binds `7` and `limit: 3000000000` binds
    `int64(3000000000)`. The parameter is an `int`, obeys the
    parameter rules of §predicates, is never optional (`:limit?` is an
    error, §predicates), and names no other parameter of
    the select; it joins the select's parameters after every
    parameter the projection, the `where` clause and the other
    settings read, so the signature ends with it: `func
    PointLatest(since int, limit int) rt.Query[Point]`. The order
    MUST be **total**, so the rows kept are the same whatever plan
    SQLite picks, an index added or not: it names a key no two rows
    share. A table's key is every column of its primary key, a column
    declared `unique`, or the columns of a unique index over columns
    alone without `where:` (§indexes), none of them `null`, since
    NULLs repeat under `unique`. Without `group:` a row is a row of
    the `for` table, so the order names a key of it, of every member
    of a group, as its own columns, and a joined relation of many
    rows, a reverse relation (§relationships-ref), repeats that row
    for each of its own, so the order names a key of that relation's
    table too, qualified (`comments.id`); an exists term
    (§predicates) repeats no row and asks for no key. With `group:` a row is a
    group, so the order names every `group:` item, or a key of the
    `for` table that `group:` holds as its own columns when no `group:`
    item reaches a relation of many rows, one group then being one row
    of the table. An alias of a lone column (rule 9) is that column
    here, in `order:` and in `group:` alike: `(id as ident, ...)`
    ordered by `ident` names `id`. The key may stand anywhere in the
    list, and a table with no key takes no `limit:`. Each of
    these is an error: `limit:` without `order:`, or over an order
    that leaves ties (`order: (value desc)` alone), since the rows kept
    at the cut would be arbitrary; a literal below 1, since `LIMIT 0`
    keeps nothing and SQLite reads a negative limit as none, a literal
    that does not fit `int64`, and any other value; `limit:` on a
    select that yields one row (rule 10), which has nothing to cap; and
    a second `limit:`, since a select keeps one count of rows. The statement
    ends in `LIMIT ?` after `ORDER BY`, the literal or the parameter
    bound behind it (§predicates, rendering): each member's statement
    of a group takes it, after `GROUP BY` it counts groups, and under
    `with` it is the parent statement's, so it counts parents and each
    keeps all its children. At run time a parameter below 1 makes the
    function's effect carry `rt.ErrLimit` in `Err` (`Parent.Err` under
    `with`), which the runner returns without running a statement
    (§generated-query-surface): SQLite reads `LIMIT -1` as no limit at
    all and would return every row. A value from a request never gets
    that far: a query route and a route load refuse one below 1 where
    they bind it, a 400 naming the key (§query-routes, §route-loads),
    and a load's literal below 1 is an error (§route-loads), so an
    `rt.ErrLimit` that reaches the error spine is a value the
    application computed, and its 500 (§formats). The generated client
    method takes the parameter (§generated-client).

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
`partial body` (§table, §tablepartial); a body MAY contain more than
one, and their lines accumulate in source order, an injected partial's
at its injection (§partial-injection-order).

```volt
table users {
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
   package**, bare (`EmailShape`, §imports); an imported package's
   function is an error suggesting a local wrapper, and the package's
   own name as a qualifier (`db.EmailShape`) is an error naming the bare
   form. The function MUST exist as an exported,
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
   can close against concurrent writers, and they reach the caller of
   `Create`, `Update` or `Delete` as `*rt.Invalid` (§changesets).
6. **Generation contract.** `Validate` is the changeset's (§changesets):
   `volt_validate.go` declares `func (c <Model>Changeset) Validate()
   <Model>Changeset` for every table, which runs rule 4 over `Row()`
   and records each failed validator as a field error whose check is
   the validator's name, whose columns are the ones it reads and whose
   message is the returned error's. Nothing else calls a validator;
   `Create` and `Update` call `Validate` before writing.
7. **Statuses and attribution.** A refused `Create`, `Update` or
   `Delete` is one `*rt.Invalid` itemizing every field error
   (§changesets), which the error spine of the network layer answers
   with 422 for failed validators and 409 for a unique violation and
   renders into the error body (§formats) so a form can mark the fields.
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
nothing happens if the changeset is invalid. Every generated insert
and keyed update takes a changeset; `Delete` and the bulk paths of
rule 11 are the only other writes.

```go
type UserChangeset struct{ rt.Changeset[User] }  // the generic changeset plus the typed setters
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

func UserCreate(c UserChangeset) rt.Write[User]   // the insert, as a value the runner performs
func UserUpdate(c UserChangeset) rt.Write[User]   // the update, likewise
```

```go
cs := user.Changeset().Cast(params, db.UserEmail, db.UserBio).Validate()
if !cs.Valid() {
	return cs.Err()
}
user, err := run.Run(ctx, db, db.UserUpdate(cs))
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
   database-assigned key (§columns) does until the database assigns
   it. An unmarked NOT
   NULL column without a default reads as its zero value too, but the
   insert names no value for it, so the database writes NULL and
   refuses the row as a `not_null` field error (rule 8), unless
   `Validate` refused that zero value first. The one NOT NULL column
   SQLite fills rather than refuses is the rowid alias, a
   single-column primary key stored as `int` (§columns,
   §mapping-to-sqlite-ddl): left unmarked, it gets a rowid SQLite
   picks. That is the design for the database-assigned key. Any other
   rowid alias is an ordinary key, a shared key that is also a foreign
   key or a key of a declared type stored as `int`, and a rowid SQLite
   picked is a value the row never gave, one that may name a row of
   the referenced table nobody chose, so `rt.Insert` refuses a
   changeset that leaves it unmarked with a `not_null` field error on
   the column, the error an ordinary NOT NULL column without a default
   gets from the database, before any statement runs; it adds none
   when an error on that column is recorded already, as `required`'s
   is.
   `u.Changeset()` starts from `u` with nothing set and
   remembers that it was made from a row. A setter records a value
   for one column and **marks** it; `Changed` reports the mark; a
   column set to the value it already holds is still a change, since
   the changeset records intent, not difference. `Row()` is the data
   with the marked values applied. `From(u)` marks every data column
   of `u` but the database-assigned key, so a zero row never inserts
   the id 0, and marks a shared key that is also a foreign key, whose
   value is the referenced row's; the database-assigned key has a
   setter as every column does, since SQLite stores an explicit rowid,
   and a changeset that marks it inserts the value given.
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
   the client cannot widen it. A key of `p` that is no column's wire
   name (rule 5) is a field error with check `unknown_field`, so a typo
   never vanishes; a column renamed by its `json` tag is known by the
   tag's name alone. The database-assigned key is set like any column
   when `allow` lists it; the generated create of a query route leaves it
   out of `allow` (§query-routes), so a body sent there that carries
   it is neither an error nor a change.
5. **`Params`.** `rt.Params[M]` is a write body as the wire carries
   it, in either format (§formats): as JSON, an object of the changed
   columns keyed by their wire names (the `json` tag, §struct-tags),
   each value as `encoding/json` renders it and `null` for NULL, but a
   nil `[]byte` as `""`, since nil is empty bytes (§columns), and a
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
   ones the check reads, and for `unknown_field` the key as sent.
8. **`Create` and `Update`.** `<Model>Create(c)` is the `rt.Write`
   that validates and, if valid, inserts the marked columns and yields
   the stored row when the runner performs it (§generated-query-surface);
   a column never marked takes its database default or, lacking one,
   NULL, which a `NOT NULL` column refuses as a `not_null` field error
   (rule 10), a single-column primary key stored as `int` excepted,
   which the database assigns (rule 2); nothing marked inserts
   `DEFAULT VALUES`.
   `<Model>Update(c)` is the `rt.Write` that validates and, if valid,
   issues `UPDATE <table> SET <the marked columns> WHERE <the key of
   the row the changeset was made from> RETURNING` and returns the
   stored row; nothing marked touches nothing and yields `Row()`;
   a changeset not made from a row (`UserNew()`) is `rt.ErrNoRow`,
   there being no key; a vanished row is `rt.ErrNotFound`. Both
   statements are rendered from the marks when the function is called
   and prepared through the runner's statement cache, like the dynamic
   layer. A Write built from an invalid changeset carries its
   `*rt.Invalid` and the runner returns it without running a
   statement; `Update` carries `rt.ErrNoRow` for a changeset not made
   from a row before validity.
9. **`required`** (§columns) is the one built-in rule of `Validate`
   (rule 6), which the schema switches on per column: it refuses the
   type's empty value, `""` for `string`, `0` for `int` and
   `float64`, a zero length for `[]byte`, and for a declared type the
   empty value of its storage type, compared directly for a kinded
   type and read through the type's `driver.Valuer` otherwise
   (§types). It is Go only, like every validator.
10. **`rt.Invalid`.** Every refused `Create`, `Update` or `Delete` is
    one `*rt.Invalid{Model, Fields, Cause}`: the field errors of the changeset when validation
    failed, or the database's refusal mapped onto fields when the
    statement ran: a unique violation becomes a field error with
    check `unique` on the index's columns (on none for a unique index
    with an expression atom, which constrains no column, §indexes, and
    which SQLite names by the index alone), a NOT NULL violation
    `not_null` on its column, a foreign key violation `foreign_key`
    with no column (SQLite names none), each wrapping the driver's
    error as `Cause`. It carries no HTTP status, the data runtime
    knowing nothing of HTTP (§data-and-network-layers): the error spine
    answers 409 when a `unique` field error is among its fields and 422
    otherwise (§formats). It itemizes itself (`Details()`: check,
    columns, message), which the error spine renders into the error
    body so a form can mark the fields. Any other database
    error is returned as is, and the bulk paths of rule 11 return the
    driver's error unmapped.
11. **Bulk paths bypass.** `UpdateWhere` and `DeleteWhere` of the
    dynamic layer take assignments and predicates, never a row, and
    run no validator; `Delete` runs none either. They are the
    equivalent of raw SQL and are documented as such.
12. **Generation contract.** Everything algorithmic lives in the
    runtime's `rt.Changeset[M]`: the marks, `From`, `Cast` with its
    `cast` and `unknown_field` errors, `Changed`, `Row`, `Params`,
    `ErrorAdd`, `Errors`, `Valid`, `Err`, `FromRow`, which reports
    whether the changeset was made from a row, and the rendering of
    the insert and the update. What is generated is what reflection
    would otherwise do. `volt_models.go` declares per model the
    changeset type `<Model>Changeset struct{ rt.Changeset[M] }`, a
    defined type embedding the generic one so it can carry the typed
    setters; `<Model>New` and `Changeset`, which make a changeset over
    the model's desc, the only way one is made; `From`, `Cast` and
    `ErrorAdd` as one-line wrappers returning `<Model>Changeset` so a
    chain keeps its setters; the setters, each marking its column
    through `rt.Set`; the aliases `<Model>Column` and `<Model>Params`;
    the column handles `<Model><Field>` (`rt.Column[M, T]`,
    `rt.NullColumn` when nullable); and the unexported desc
    `volt<Model>Desc`, an `rt.Desc[M]` carrying the model's name, its
    quoted table and column list, one `rt.FieldDesc` per column
    (column, wire name, key, rowid alias, database-assigned key), the row of literal defaults (rule 2), and the generated
    functions `volt<Model>Scan` (a row from `rt.Row`),
    `volt<Model>Values` (a row's bound values in column order),
    `volt<Model>Apply` (a row with the marked values applied) and
    `volt<Model>Cast` (one column of a write body, through the typed
    setter). So a program that shares the models, a generated client
    among them, builds changesets with nothing else.
    `volt_validate.go` declares `Validate`, which is where the
    package's validator functions are called and which replaces the
    findings through `WithFindings`. `volt_queries.go` declares `Create`
    and `Update`, over `rt.Insert` and `rt.Update`, which render
    through the changeset's `InsertRender` and `UpdateRender`. The names
    `<Model>Changeset`, `<Model>Column`, `<Model>Params` and
    `<Model>New` join the package-scope names the `dynname` lint and
    §selects-over-groups guard.

---

## Migrations

A **migration** is a named list of structural operations, written by
hand, for instance from the operations the drift report prints
(§the-fold). Migrations are
separate from the schema: the schema says what the database is, a
migration says how one database becomes the next. A migration is not
checked against the schema operation by operation, since it may say
anything that was once true; the composition of all of them is
(§the-fold).

```ebnf
migration          = "migration", migration name, [ migration settings ],
                     "{", migration body, "}" ;
migration name     = digit, digit, digit, digit, { digit }, "__", label ;
label              = ( letter | unicode digit ), { letter | unicode digit } ;

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

exec op            = "exec", expression literal, [ exec settings ], newline ;
exec settings      = "[", "down", ":", expression literal, "]" ;

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
a `ref` and a migration has no `ref`. The
words of a migration body (`up`, `down`, `create`, `drop`, `rename`,
`alter`, `rebuild`, `exec`, `table`, `index`, `add`, `to`) are
keywords inside a `migration` only; elsewhere they are identifiers.

```volt fragment
migration 0002__users_bio {
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
   digits, two underscores and a label; it is one token, the only name
   that begins with digits, and no identifier (§identifiers). It MUST be
   unique within the package. Every
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
   and a key column MUST NOT carry `null`. A `shape body` follows
   §columns and §indexes as a table does: its `pk` columns form one
   primary key in declaration order, a single-column index is the
   column's `unique` or `index`, its `indexes` block holds composite,
   expression and partial indexes, no two of its indexes share a
   name, and a single `int` key is the rowid alias, which takes no
   `default:`, and the database-assigned key unless it carries `ref:`,
   which alone takes `autoincrement` (§columns). A partial injection, a `validators` block and a `records`
   block are not part of a `shape body`.
4. **Settings whitelist.** In a migration, a column setting is one of
   `pk`, `autoincrement`, `null`, `unique`, `index`, `default:`, `ref:`,
   `delete:`, `update:`, and in a `rebuild` also `from:` (rule 9);
   `from:` anywhere else is an error.
   `ref: table.column` is spelled as in the schema (§columns): the
   declaring column holds the key. `delete:` and `update:` take the referential actions of
   §relationships-ref (`cascade`, `restrict`, `set_null`,
   `set_default`) for the column's `ref:`, so that a schema
   foreign key with an action can be matched; without a `ref:` they
   are errors. As there, `set_null` requires the column to carry
   `null`, and `set_default` requires `null` or a `default:`. An index
   setting is one of `unique`, `name:`, `where:`; `pk` on an index is
   an error naming the column form, as in §indexes. `tag:`, `required` and
   `model:` describe Go; all three are errors here, in every position
   of a migration. A doc comment in a migration documents its construct
   to the reader and generates nothing (§doc-comments). A migration takes `irreversible` and
   `no_transaction` and no other setting; an `exec` takes `down:` and
   no other setting.
5. **`create table`.** The body MUST contain at least one column;
   names are unique within it. It lowers by the DDL mapping exactly
   as the schema's `table` does (§mapping-to-sqlite-ddl), index names
   included: the `CREATE TABLE`, then the `CREATE [UNIQUE] INDEX` of
   every column that carries `unique` or `index`, then those of its
   `indexes` block.
6. **`drop table`.** Drops the table and, with it, its indexes. The
   optional body is the shape being dropped and exists only so that
   the migration is reversible (rule 12); when written, it MUST
   contain at least one column, as a `create table` body must.
7. **`rename table`.** `ALTER TABLE … RENAME TO`. Foreign keys that
   reference the table follow it; index names do not, and are kept
   as they are. The fold compares indexes by key, never by name
   (§the-fold), so a kept name is no drift; a later `drop index` of
   such an index gives its old name with `name:`.
8. **`alter table`.** Each item lowers to its `ALTER TABLE` statement,
   in the written order; an `add` or `drop` of a column that carries
   `unique` or `index` also lowers to that index's statement, and an
   `add` whose default is an expression to the five statements of its
   schema edit, as below. `add` is `ADD COLUMN` under SQLite's rules,
   checked here rather than at run time: a column without `null` MUST
   carry a `default:`; `pk` and
   `autoincrement` are errors, since a key and `AUTOINCREMENT` are
   part of the table's definition, which only a rebuild rewrites; `ref:` requires the column to carry `null`;
   a column with `ref:` takes no `default:`, since SQLite refuses to
   add a `REFERENCES` column with a default other than NULL to a table
   that has rows; a column that carries `unique` or `index` lowers to
   the `ADD COLUMN` followed by its `CREATE UNIQUE INDEX` or `CREATE
   INDEX` under the derived name (§indexes), so a unique column is
   added without a rebuild, where SQLite's own `ADD COLUMN ... UNIQUE`
   is refused, and its inverse drops the index before the column.
   A column whose `default:` is an expression literal is added
   without a rebuild too, although SQLite's `ADD COLUMN` refuses a
   default that is not a constant (`CURRENT_TIMESTAMP`) on a table
   that has rows. It lowers to five statements, run inside the
   migration's transaction: the `ADD COLUMN` of the column without
   its default, NOT NULL with the placeholder `DEFAULT 0` unless it
   carries `null`; `UPDATE <table> SET <column> = <default>`, which
   writes the default's value into every row, so that a NOT NULL
   column whose default yields NULL fails the migration here;
   `PRAGMA writable_schema = ON`; an `UPDATE sqlite_schema` that
   replaces, in the table's `CREATE TABLE` text, the definition the
   `ADD COLUMN` appended with the column's own; and `PRAGMA
   writable_schema = RESET`, which reloads the schema; then the
   column's `CREATE INDEX`, as above. This is SQLite's documented
   procedure for changing the default of a column. It is sound
   because the `UPDATE` stores the column in every row, so no row
   ever reads the default from the definition, and because the `ADD
   COLUMN` advanced the schema version in the same transaction, so
   every other connection reads the edited definition once the
   migration commits. A `no_transaction` migration has no
   transaction to hold the five, so an expression default there is
   an error in an `add` and in the shape of a typed `drop` of a flat
   body, which is its inverse `add`. A connection in SQLite's
   defensive mode refuses the edit, and the migration fails and
   rolls back.
   `drop` is `DROP COLUMN`; the
   optional type and settings are the dropped column's shape, which
   in a flat body is the inverse `add` and MUST satisfy the `add`
   rules above (rule 12), and inside an `up { }` or `down { }` block
   is checked as a column only, since there the inverse is written
   out. A `drop` whose shape carries `unique` or `index` lowers, in a
   flat body as in a block, to the `DROP INDEX` of that index under
   the derived name, then the `DROP COLUMN`, since SQLite refuses to
   drop an indexed column: the mirror of `add`, so a typed `drop` is
   the inverse of its own inverse. A `drop index` of that index written
   before it would drop the index twice, which the fold refuses
   (§the-fold). `rename` is `RENAME COLUMN`.
9. **`rebuild table`.** The one operation for every change SQLite
   cannot `ALTER`: a type, a nullability, a default, a primary key,
   `autoincrement` added or removed, a foreign key. The body is the new shape and MUST contain at least
   one column. It lowers to: create `<table>__new` from the body;
   `INSERT INTO <table>__new (<cols>) SELECT <sources> FROM <table>`;
   when the body's key carries `autoincrement`, the two statements that
   give `<table>__new` the larger of its own `sqlite_sequence` mark and
   the old table's, so the high-water mark survives the rebuild and no
   deleted id is assigned again (§columns), while a rebuild that adds
   `autoincrement` starts the mark at the largest copied id, since a
   table without it keeps none;
   `DROP TABLE <table>`; `ALTER TABLE <table>__new RENAME TO
   <table>`; create every index of the body, those of its columns'
   `unique` and `index` first, named for `<table>`. The source of a column
   is, in order: its `from:` expression literal if given; the old
   column of the same name; its `default:`, which the rowid alias never
   has (§default-values); `NULL`. A column without `null` whose source
   is `NULL` is an error, except the database-assigned key (`int
   [pk]`, with or without `autoincrement`, without `ref:`) introduced
   by the rebuild, which takes `NULL` as its source and the database
   assigns it; a rowid alias that carries `ref:` and has no old column
   of its name needs a `from:`, since `NULL` would let the database
   assign it ids the referenced table never held. The source order
   is checked here for a table the package's migrations created; a
   table of unknown shape takes the old column of the same name when
   the fold has it and otherwise errors at fold time (§the-fold).
   Nothing but the body survives the rebuild: an index not restated
   in it is gone. A rebuild of a table that a view references, or
   that a trigger on another table references, needs an `exec`
   dropping and recreating them around it; without it the fold stops
   at the rebuild's final rename with SQLite's message and that hint.
   A trigger on the rebuilt table itself is dropped with the table and
   nothing reports it, since the fold compares no triggers; an `exec`
   after the rebuild recreates it.
10. **`create index`, `drop index`.** The index is identified by its
    table and key as in a schema `indexes` block, so the schema and
    the migration name the same index the same way: the DDL mapping
    assigns the name from the table and the key columns, `name:`
    overrides it, and an index with an expression atom MUST carry
    `name:` (§indexes). `where:` takes an expression literal and makes
    the index partial. These operations take a key of one plain column
    too, since `create index t (col) [unique]` is how an existing
    column gets the index its `unique` or `index` setting declares, and
    `drop index t (col)` drops it, under the name the setting derives,
    `idx_<table>_<col>`. `pk` is no index setting: a primary key is a
    table constraint, declared by `pk` columns in a `create table` or
    `rebuild table` body.
11. **`exec`.** The expression literal is one or more SQL statements
    run verbatim, raw as every expression literal is
    (§expression-literals), so a SQL string inside keeps its single
    quotes, for whatever the operations above do not say: a data
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
   and position, the column's place in the key's column list
   (`PRAGMA table_xinfo`), so that a composite key in another order is
   a rebuild (§columns), and whether the key carries `AUTOINCREMENT`,
   which no PRAGMA reports and which is read from the table's `CREATE
   TABLE` text in `sqlite_schema` outside its quoted names, strings and
   comments, so that adding or removing `autoincrement` is a rebuild;
   its foreign keys
   (`PRAGMA foreign_key_list`); its indexes with uniqueness, partial
   predicate and key (`PRAGMA index_list`, `PRAGMA index_xinfo`).
   Indexes are compared by table, key, uniqueness and predicate,
   never by name, each index of one side matched with at most one of
   the other: an index renamed is the same index, and the index of a
   `UNIQUE` constraint (SQLite's autoindex, which no `DROP INDEX`
   removes) is the unique index on its key, so an inline `UNIQUE` that
   an `exec` writes folds equal to the `CREATE UNIQUE INDEX` of a
   `unique` column (§mapping-to-sqlite-ddl). A constraint in the fold that the target
   lacks (an `exec` can write one) is closed by a rebuild, since only a
   rebuild removes it. The equality is structural only. The fold is
   the migrations, never a live database: a database changed by hand,
   whose structure differs from the fold, can refuse a migration the
   check accepted. Nothing else
   is compared: not index names, not column order, not row data, not
   SQL text beyond `AUTOINCREMENT`, not the order of statements, never
   the ledger table (§migration-ledger), and never a table whose name
   begins with `sqlite_`, SQLite's own, among them the `sqlite_sequence`
   that appears as soon as one table carries `AUTOINCREMENT`.
3. **Drift.** `volt migration check` compares fold and target and
   reports every difference, table by table, as the operations that
   would close it. Zero differences is the success condition. `volt
   gen` runs the same check for a package that declares at least one
   migration and fails on drift, so a schema edit without its
   migration never generates; a package without migrations is
   unmigrated, and `gen` runs no fold for it.
4. **Lemma.** Because the fold runs on the engine that will run the
   migrations, a migration SQLite would refuse (dropping an indexed
   column, say) is refused at check time, with SQLite's own message.
   The fold's database is empty, so a statement that only existing
   rows can make fail passes the check and may still fail at run time:
   a `rebuild table` that copies NULL into a column without `null`, a
   unique index over duplicate values, a foreign key the rows break
   (the `foreign_key_check` of §migrations), an `add` of a column
   without `null` whose expression default yields NULL, which the
   backfill of that `add` refuses (§migrations). One such case is
   closed before run time: an `add` of a column without `null` MUST
   carry a `default:` (§migrations), since SQLite refuses that column
   on a table with rows. Once the fold is read, the check also reverts the
   reversible migrations above the last irreversible one, from the
   top down, and applies them again in the fold database, so an
   inverse SQLite would refuse is refused at check time as well; a
   down cannot pass an irreversible migration, so the migrations
   below it are not run backward.
5. **Generation.** The drift report is the input of a migration
   generator (not built): it writes the operations that close the
   drift as a new `migration` for the user to edit. A rename is a
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
   codes, the same from `gen`: 1 for drift, for a package the checker
   refuses (each command checks its package first and prints the
   diagnostics, as `volt check` does) and for an error in the
   migrations themselves, a statement SQLite refuses in either
   direction, an irreversible migration asked to go down, a migration
   that does not lower; 2 for usage and I/O, a database that cannot
   be opened, is locked past its busy timeout or fails otherwise. The
   tool carries its own SQLite (the `mattn/go-sqlite3` driver) and
   needs cgo, a C compiler at build time, for it; a `volt` built
   without cgo refuses `migration check`, `up`, `down` and `status`,
   and `gen` for a package that declares migrations, with exit 2 and a
   message saying so; `migration new` and `migration sql` need no
   SQLite. The library and the runtime carry no SQLite.
2. **Generated.** For a package that declares migrations, `volt gen`
   writes `volt_migrations.go` beside the other generated files:
   `var Migrations = []rt.Migration{…}` in name order, each with its
   name, up statements, down statements (nil when irreversible) and
   its transaction and foreign-key bracket flags, and nothing else:
   the application applies them with `run.MigrateUp(ctx, db,
   Migrations)` (rule 3). The file belongs to the
   queries part (`-parts queries`) and is absent with `-parts
   models`. With it, a binary migrates its own database and a test
   builds a fresh one with one call, without the `volt` tool or the
   `.volt` files.
3. **Runtime.** `rt.Migration` is the statement form of one
   migration, a value; the runner applies it over a `run.DB`
   (§generated-query-surface): `run.MigrateUp(ctx, db, migrations)`
   applies the pending ones and returns their names;
   `run.MigrateDown(ctx, db, migrations)` reverts the last applied one
   and returns its name; `run.MigrateStatus(ctx, db, migrations)` lists each migration with
   whether and when it was applied and whether it is out of order,
   and every ledger row that no migration of the package names,
   marked as such. The CLI's `up`, `down` and `status` are these
   three over the statements lowered from the `.volt` files. A
   migration that does not run, a statement SQLite refuses, a failed
   foreign-key check or a down asked of an irreversible migration, is
   an `*rt.MigrationError` carrying the migration's name, the direction
   and the cause, and for a statement SQLite refuses also its position
   and text, so that a caller tells it from the database failing.
4. **No implicit schema change.** Neither `volt gen` nor the runtime
   ever alters a database from the schema alone. The schema is the
   target; a migration is the only path to it.

```volt fragment
package db

table users {
  id        int    [pk]
  email     string [unique, required]
  full_name string [required]
  bio       string [null]
  hits      int    [default: 0]

  validators {
    EmailShape(email)
    HitsSane(hits)
  }
}

migration 0001__create_users {
  create table users {
    id    int    [pk]
    email string [unique]
    name  string
    hits  string [default: "0"]
  }
}

migration 0002__users_bio_and_int_hits [irreversible] {
  alter table users {
    add bio string [null]
    rename name to full_name
  }
  rebuild table users {
    id        int    [pk]
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

pipeline       = "pipeline", name, "{", { plug }, "}" ;
plug           = "use", go ref, newline ;
go ref         = name, [ ".", name ] ;

scope          = "scope", route path, [ settings ],
                 "{", { scope item }, "}" ;
scope item     = route | resources | dataset | scope | scope load ;
scope load     = "load", name, go ref, newline ;

route          = verb, route path, handler ref, [ settings ],
                 [ load block ], newline ;
verb           = "get" | "post" | "patch" | "delete"
               | "options" | "head" | "any" ;
handler ref    = name, [ ".", name ] ;
load block     = "{", { load }, "}" ;
load           = name, query call, newline ;
query call     = query ref, "(", [ arg, { ",", arg } ], ")" ;
query ref      = name, [ ".", name ] ;
arg            = name | literal ;

route path     = slash, [ segment, { slash, segment } ] ;
segment        = name
               | ":", name, [ "(", param type, ")" ]
               | ":", name, "." , ".", "." ;
param type     = "int" ;

resources      = "resources", table ref, [ settings ], newline ;
table ref      = name, [ ".", name ] ;
dataset        = "dataset", [ name, "." ], name, [ settings ], newline ;

migration          = "migration", migration name, [ migration settings ],
                     "{", migration body, "}" ;
migration name     = digit, digit, digit, digit, { digit }, "__", label ;
label              = ( letter | unicode digit ), { letter | unicode digit } ;
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
exec op            = "exec", expression literal, [ exec settings ], newline ;
exec settings      = "[", "down", ":", expression literal, "]" ;
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
setting value  = (* schema-layer alternatives *) | ident list | order list
               | query ref | query call | param ;
ident list     = "(", name, { ",", name }, ")" ;
```

The `group decl`, `pred decl` and `select decl` productions — with the
predicate expression grammar they share — are collected in §groups, §predicates
and §selects-over-groups, and the validators block in §validators; they are not
duplicated here. The `order list` that extends `setting value` above is the
value of a Select's `order:` and is defined in §selects-over-groups; the
`param` that extends it is the value of a Select's `limit:` and is
defined in §predicates.

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
   them (a token dropped, inserted, or two exchanged) MUST be decided
   the same way by the front end: inside the grammar it parses, and
   outside the front end reports an error it does not report for the
   sentence the text came from. The grammar
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

*The data layer: generation contract of `volt gen` for schema packages (`volt_models.go`,
`volt_queries.go`, `volt_dyn.go`). Informative in form; in force it is
pinned by the generator goldens, which are gofmt-stable and compiled by
the real Go toolchain on every `go test ./...` run.*

`volt_models.go` carries every type the wire carries that Volt
generates: the row structs, the changesets with their column handles,
their descs and `<Model>Params` (§changesets, generation contract), so a program
that shares the models, a generated client (§generated-client) among
them, needs nothing else but the declared types the models name: a
package-qualified one from its own package, a bare one from the data
package's Go files (§types).
`volt_queries.go`, `volt_dyn.go` and `volt_selects.go` carry the query
functions (§generated-query-surface), and import only what their
signatures name.

### Go types

A column type is a Go type (§columns), so the mapping is the identity
for the built-ins; the checker has already refused anything outside
the set.

| Declared | Go |
|---|---|
| `bool`, `int`, `float64`, `string`, `time.Time` | the same |
| `[]byte` | `[]byte`, bytes and never NULL: nil is empty bytes (§nullability) |
| declared `Type pkg.Name` (§types) | `pkg.Name`, imported as `import pkg "path"` |
| declared `Type Name` (§types) | `Name`, the data package's own type, named unqualified |

`int` is Go's `int`: Volt targets 64-bit platforms (amd64 and arm64),
so a route parameter or primary key of type `int` carries the full
SQLite INTEGER range there, and only there.

A `time.Time` has one stored form: SQLite's own datetime text in
UTC, `YYYY-MM-DD HH:MM:SS`, followed by `.` and the fractional
seconds without their trailing zeros only when the fraction is not
zero (`2026-11-03 05:18:49`, `2026-11-03 05:18:49.5`). In Go it is
`t.UTC().Format("2006-01-02 15:04:05.999999999")`, the layout
`rt.TimeLayout`, written by `rt.TimeText`. It is the text
`CURRENT_TIMESTAMP` and `datetime()` write, whole seconds; it is not
what `datetime(..., 'subsec')` and `strftime('%f')` write, which is
always three fractional digits (`12:00:00.000`, `12:00:00.120`), so a
default or record value is held to the expressions that write it
(§default-values). A default, a value written from Go and a value read
back and bound again are therefore one text;
and since every value spells its seconds in the same width and a
shorter fraction sorts before a longer one, text order is instant
order. Equality, the ordering comparisons, `in` and list parameters,
`order:`, `min` and `max`, the key of a `with`, keyset paging with
`After` and SQLite's date and time functions (`date(created_at)`)
therefore all agree with the instants.

The runtime binds every `time.Time` as that text, whatever its
location, and never hands a `time.Time` to the driver: every argument
of every statement the runner executes passes through
`rt.ArgsBound`, which rewrites a `time.Time` and any `driver.Valuer`
whose value is a `time.Time`, so a changeset write, a key path
(`Get`, `GetMany`, `Update`, `Delete`), an operator argument, an `In`
item, a `Set`, an `After` key and a `Raw` argument bind it alike; a
valid `rt.Null[time.Time]` binds as that text itself, a declared type
stored as `time.Time` through its own `Value` and then as that text,
and every item of a list parameter or an `InList` travels in the JSON
array as that text (`rt.JSONList`, and `rt.JSONListOptional` for an
optional list, §predicates). A year outside 0000
to 9999 in UTC, past which SQLite's date functions and the text order
end, is an error, the same one on every path: the statement returns
it without running, an `In` item, a list parameter, an `InList` and
the parent keys of a `with` alike (`rt.JSONList` returns an
`rt.List` that carries the error to the statement instead of
panicking, as it does for an item whose `Value` fails), whether the
value came from Go or was read back from a row. A query parameter
outside those years, such as `9999-12-31T23:00:00-05:00`, which is
year 10000 in UTC, is a 400 naming the parameter (§query-routes).

A value reads back in UTC. A NOT NULL `time.Time` field, of a model or
of a select's row, scans through `(*rt.Time)(&v.At)`, and
`rt.Null[time.Time]` reads the same way: the driver's `time.Time` for
a declared column; for `min` or `max` of one, which carries no
declared type for the driver to convert by (§selects-over-groups),
SQLite's text in any layout SQLite or the driver writes (a `T` or a
space, a trailing `Z` or an offset, with or without seconds, or a
date alone) and an integer as the driver reads one from a declared
column, unix seconds, or unix milliseconds when its magnitude passes
10^12, so a column and `min` of it read one row alike. NULL in a NOT
NULL field, text in no such layout and any other value are errors.

Two limits remain. The sqlite3 driver converts a column declared
`TIMESTAMP` itself and reads text it cannot parse as the zero
`time.Time`, without any other sign, so such text in a declared
column reads as `0001-01-01 00:00:00`, which is also a value Volt can
store, and Volt cannot refuse it there; `min` and `max` of the
column refuse it. The limit is Volt's choice of DDL spelling
(§mapping-to-sqlite-ddl), not SQLite's: every generated read scans
through `rt.Time`, which refuses such text, so a spelling the driver
does not convert would make the refusal reliable, but it would also
take the `time.Time` away from code that reads the column through
`database/sql` itself, and only SQL written by hand stores such text.
A row stored in another form, by older code that bound a `time.Time`
with its offset (`2026-03-01 14:00:00+02:00`) or by SQL written by
hand, reads back as its instant in UTC, but SQLite compares only the
stored text: such a row does not equal its own read-back value, and
it orders among canonical rows by its text. Volt generates no
migration for such rows. In a column that is no key, reading the row
and writing its value back (`Update` with the field set, or
`UpdateWhere` with a `Set`) stores the canonical form. A key column,
and a foreign key that references one, is out of the generated API's
reach: every key path binds the canonical text, so `Get`, `Update`
and `Delete` by the read-back key find no row, and only a statement
written by hand rewrites it (`UPDATE batches SET opened_at =
datetime(opened_at)` for whole seconds, its children's keys alike).

### Nullability

A column that carries `null` (§columns) is nullable and generated as
`rt.Null[T]`, a value plus a validity bit, never a pointer, for every
type, `[]byte` included (`rt.Null[[]byte]`). Every other column is NOT
NULL and generated as `T`. A valid `rt.Null[time.Time]` binds as the stored form of a `time.Time`
and reads as a NOT NULL one does, NULL aside (§go-types). A plain
`[]byte` is therefore bytes and
never NULL (§columns), and `rt.Bytes` is the one place that says
so: generated code binds a `[]byte` field as `rt.Bytes(v.Body)`, whose
`Value` writes nil as empty bytes, in a changeset write and on every
key path (`Get`, `GetBy*`, `Update`, `Delete`), and scans it through
`(*rt.Bytes)(&v.Body)`, which reads empty bytes as a non-nil empty
slice even when the driver hands back nil, and NULL as an error; the
dynamic layer binds every plain `[]byte` value through it, an
operator argument, an `In` or `InList` item (§predicates), a
`Set`, an `After` key and a `Raw` argument alike. `rt.Null[[]byte]`
follows the same rule when valid: a nil `V` binds as empty bytes, a
read is never nil, and JSON renders a valid nil `V` as `""` and NULL
as `null`, reading `""` back as valid empty bytes and `null` as NULL.
A plain `[]byte` field renders in JSON as `encoding/json` renders it,
so nil renders as `null`: a row read from the database holds non-nil
bytes and renders empty bytes as `""`, while a zero row, a
changeset's `Row()` or a row the generated client decoded from GOB
(§formats) holds nil and renders `null`, which `Cast` refuses
on a column without `null` (§changesets). GOB does not carry an
empty slice, so empty bytes in a GOB write body (§formats) arrive as
nil, which is empty bytes again: stored as empty bytes in either kind
of column, while NULL arrives as an invalid `rt.Null` and is stored as
NULL. A declared type stored as `[]byte` binds and reads through its
own methods (§types). A nullable
column's handle, declared in `volt_models.go` with the rest
(§changesets, §generated-query-surface), is an `rt.NullColumn`:
comparisons and `Set` take plain `T`, NULL is explicit (`IsNull`,
`SetNull`), so `Set(nil)` on a `[]byte` column assigns empty bytes.

### Names

`snake_case` → `PascalCase` with the Go initialisms convention
(`user_id` → `UserID`, `api_key` → `APIKey`). Model names are the
singular of the table name (`users` → `User`), overridable with
`[model:]` — the only model-naming override; `resources`' `singular:`
setting (§settings) renames the route member helper, never the model.
A name whose words begin with a digit once its underscores are
dropped is prefixed (`_2fa_codes` → `X2faCode`). Two declarations mapping to the
same Go identifier is a generation error; the `dynname` lint reports
the dynamic layer's concatenation collisions ahead of time. A bare
declared type shares this scope, since the data package's own Go
files declare it. Its name is exported, so the unexported names of
generated code never meet it, and one that takes an exported
generated name, a router's included, is a check error (§types).

### Doc comments in generated code

A doc comment (§doc-comments) is carried, as its doc text, into the
generated Go where its construct becomes a Go declaration:

| Doc comment on | Generated position |
|---|---|
| package clause | package doc comment of `volt_models.go`, or of `volt_router.go` in a package without data |
| `table` | model struct doc comment |
| column | field doc comment, in the model and in the struct derivative of a star select (§selects-over-groups) |
| injected partial column | travels with the column; a §tablepartial override brings its own |
| `select` | doc comment of the query function of every member |
| route | doc comment of its action's method in the controller interface (§handler-references) |

Every other doc comment is the reader's alone: the controller
interface, a reverse-URL helper and a client method carry none, a
`resources` or `dataset` declaration's doc documents none of the
methods it expands to, and the DDL carries no comment but its header,
since the schema documents itself in Volt (§mapping-to-sqlite-ddl).
These are the only comments a generated file carries besides the header: a doc comment
is the maintainer's text, and the generators write no comment of
their own, so a file of an undocumented schema has no comment but the
header, and a package without a documented package clause has no
package doc comment. The header stands apart from the package doc
comment by a blank line, as Go requires of a file header.
Generated files start with the machine-readable
`// Code generated … DO NOT EDIT.` marker; `volt gen` refuses to
overwrite a file lacking it. Generated data-layer code imports the
standard library, `nao/rt` and the packages of declared types
(§types) only.

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
default `db`/`json` pair only. The children field of a `with` row type
(§selects-over-groups) is no column and carries `json:"<relation
name>"` alone, so the children travel under the relation's name as
written, `"points"` for `MetricWithPoints`'s `Points`, as every other
key is a name the schema spells.

### Generated query surface

A generated query is a value (§values-and-effects): a function of
`volt_queries.go`, `volt_dyn.go` or `volt_selects.go` with no receiver
and no context returns an **effect** of `nao/rt`, and the runner
`nao/run` performs it. Nothing runs when the function is called.
`Effect[T]` is sealed: every method it requires is unexported, so a
type of another package satisfies it only by embedding one of the
kinds below or an `Effect` itself, and is then the effect it embeds: it
performs, describes and compares as that one, and one that embeds a
nil `Effect` panics when it is performed or described, as a method of
a nil interface does. The runner performs an effect through
`rt.Perform` or `rt.PerformAny`, the only functions that hand an effect
an executor.

```go
// nao/rt: the effects. Effect[T] is sealed; these are all of them.
type Query[T any]  struct { SQL string; Args []any; Scan func(Row) (T, error); Err error }  // zero or more rows, []T
type Lookup[T any] struct { SQL string; Args []any; Scan func(Row) (T, error); Err error }  // one row, or ErrNotFound
type Write[M any]  struct { SQL string; Args []any; Scan func(Row) (M, error); Row M; Err error }  // insert or update of a changeset
type Exec          struct { SQL string; Args []any; Model string; One bool; Err error }    // rows affected
type With[P, C any, K comparable, R any] struct { Parent Query[P]; Child string; ChildScan func(Row) (C, error); ParentKey func(P) K; ChildKey func(C) K; Build func(P, []C) R }  // parents with their children, []R
func Tx(steps ...AnyEffect) Transaction   // all or nothing, results in order
func All(steps ...AnyEffect) Parallel     // at once, in order inside a transaction, first error stops, results in order
func Then[T, U any](e Effect[T], next func(T) Effect[U]) Effect[U]  // e, then the effect next makes of its result
func Map[T, U any](e Effect[T], f func(T) U) Effect[U]              // f of e's result
func Atomic[T any](e Effect[T]) Effect[T]                          // e and all it performs, all or nothing
func Pure[T any](v T) Effect[T]                                    // v, performing nothing
func Fail[T any](err error) Effect[T]                              // err, performing nothing
var ErrNoEffect error                     // an effect built over nothing: a nil effect, next or f, a Fail of nil
func Insert[M any](c Changeset[M]) Write[M]
func Update[M any](c Changeset[M]) Write[M]
func Perform[T any](ctx context.Context, x Executor, e Effect[T]) (T, error)  // e over the runner's executor
func Describe(e AnyEffect) string         // its kinds, statements and values, for a person to read
func Same(a, b AnyEffect) bool            // the same effect, as volt.Same compares continuations
func Equal(a, b any) bool                 // reflect.DeepEqual, a NaN equal to a NaN: how Same compares values

// nao/run: the one stateful object and the one performer.
type DB struct { /* the pool, the statement cache, the logger */ }
func Open(driver, dsn string) (DB, error)
func Run[T any](ctx context.Context, db DB, e rt.Effect[T]) (T, error)
func MigrateUp(ctx context.Context, db DB, ms []rt.Migration) ([]string, error)
```

```go
// generated, per table
func UserGet(id int) rt.Lookup[User]
func UserGetMany(ids []int) rt.Query[User]
func UserGetByEmail(email string) rt.Lookup[User]
func UserList() rt.Query[User]
func UserCreate(c UserChangeset) rt.Write[User]
func UserUpdate(c UserChangeset) rt.Write[User]
func UserDelete(id int) rt.Exec
func UserQuery(opts ...rt.Opt[User]) rt.Query[User]
func UserCount(preds ...rt.Pred[User]) rt.Lookup[int64]
func UserExists(preds ...rt.Pred[User]) rt.Lookup[bool]
func UserDeleteWhere(preds ...rt.Pred[User]) rt.Exec
func UserUpdateWhere(set []rt.Assign[User], preds ...rt.Pred[User]) rt.Exec
```

1. `<Model>Get`, `<Model>Update` and `<Model>Delete` exist iff the
   table has a primary key (one or more `pk` columns, §columns), one
   positional argument per key column in the key's order, which is
   the columns' declaration order, and `Update` writes by the key in
   that order; `Update` also needs a non-key column; `Create` and
   `Update` take a changeset and the Write yields the stored row
   (§changesets). `<Model>GetMany(keys)` exists for a single-column
   key; missing keys are absent, not errors; an empty input is a
   Query with no statement, which yields nil.
   `<Model>GetBy<Column>` exists per column that carries `unique` and
   is not the whole key, a member of a composite key among them; a
   partial or an expression unique index makes no column unique, so it
   gives none (§indexes). `<Model>List` and `<Model>Create` always
   exist; a table without a key gets List, Create, GetBy and the
   dynamic layer.
2. A `Lookup` yields `rt.ErrNotFound` on a miss, a statement that
   returned no row, and so does an `Update` whose row vanished and a
   keyed `Delete`, an `Exec` with `One` set, when nothing was affected.
   A `Lookup` and a `Write` read the first row their statement returns
   through their `Scan`, which receives the statement's error too, so
   that it can map a refusal; the statement is released when the
   `Scan` returns, whether it read the row, refused it unread or
   panicked. A `Query` yields an empty slice, never
   nil, unless it has no statement.
3. Every statement is rendered by the runtime (§predicates, rendering):
   `Get`, `GetBy*`, `List`, `Delete` and the select functions carry
   constants rendered at generation time, bound positionally in the
   order the text names their parameters, every argument through
   `rt.ArgsBound`, which binds a `time.Time` as its stored form
   (§go-types); `GetMany` and the dynamic
   layer render when the function is called, so a rendering that
   fails (an `After` without an `OrderBy`, a `Limit` below 1, which
   fails with `rt.ErrLimit`) is a value carrying its error in `Err`,
   which the runner returns without running a statement. A select
   whose `limit:` parameter is below 1 carries `rt.ErrLimit` the same
   way: its function sets `Err` to `rt.LimitCheck(n)`, nil for an `n`
   of at least 1 (§selects-over-groups), so a limit below 1 has one
   meaning on both paths and never reaches SQLite. `Create` and `Update` render from the changeset's marks
   and end in `RETURNING` (§changesets). The runner maps a database
   refusal of a `Write` or a keyed `Delete` onto `*rt.Invalid`;
   `DeleteWhere` and `UpdateWhere` yield the driver's error.
4. The column handles `<Model><Field>` (`rt.Column[M, T]`,
   `rt.NullColumn` when nullable) are declared in `volt_models.go`
   (§changesets); the dynamic layer in `volt_dyn.go` mints per model the
   wrappers `Limit`, `Offset`, `Distinct`, `OrderBy`,
   `After`, `Set`, and the verbs `Query`, `Count`, `Exists`,
   `DeleteWhere`, `UpdateWhere`. Predicates are values rendered by the
   one renderer (§predicates, rendering): an empty `In` is constant false,
   a zero predicate vanishes, a `DeleteWhere` or `UpdateWhere` with no
   predicate is an error, `After` is keyset pagination over the
   `OrderBy` terms. Every function of the three files shares the
   package scope with the column handles and the names of
   §changesets, so a column whose handle would take a function's name
   (`count` on `users` gives `UserCount`, `exists` gives `UserExists`)
   is refused by `volt gen`, which names both, and reported before
   generation by `volt vet`, as a wrapper's collision is (`lint.md`,
   `dynname`).
5. **The runner.** `run.Open(driver, dsn)` pins one connection and sets
   WAL, `busy_timeout = 5000` and `foreign_keys = ON` on every
   connection the pool opens, since SQLite keeps each setting per
   connection; `run.Wrap` takes a pool the application opened, with
   the settings it gave it. `run.Run(ctx, db, e)` performs one
   effect: every statement outside a transaction is prepared through
   the statement cache, keyed by its text, so identical shapes prepare
   once; a statement inside one runs on the transaction's connection
   and is prepared there each time, since a cached statement belongs
   to the pool and not to one connection. A
   `Transaction` runs its steps on one transaction that commits when
   every step succeeded and rolls back on the first error or a panic, a
   `Transaction` inside one joins it. A transaction that can write
   begins immediately (`BEGIN IMMEDIATE`), taking SQLite's write lock
   at its start, so one that reads and then writes, the check of rule
   6 among them, waits for the lock while another connection of a pool
   holds it, where a deferred one would fail at its first write with
   `database is locked`. The runner waits for that lock itself, as
   SQLite's busy handler would, for at most the connection's busy
   timeout, and stops waiting when the context ends, failing with the
   context's error: SQLite's own wait sees no context. A transaction
   whose effects can only read, a `With`, and a `Transaction`, a
   `Parallel`, a `Map` or an `Atomic` of `Query`, `Lookup`, `With`,
   `Pure` and `Fail` alone, begins deferred (`BEGIN`): it takes one
   snapshot at its first read and no lock, so under WAL it reads
   beside a writer without waiting and beside other readers at once.
   A `Then` counts as writing, since what its `next` returns is not
   known before it runs. With the one connection `run.Open` pins the
   transactions run one after another either way. A statement outside
   a transaction that finds the write lock held, by another connection
   or another process, waits under SQLite's own busy handler, which no
   context interrupts, so a route's deadline (§route-loads) reaches it
   only when the busy timeout ends; an `Atomic` around it waits as a
   transaction does. A `Parallel`
   runs its steps on
   their own goroutines and joins them, the first error cancelling the
   rest, except inside a transaction, where it runs them in order on
   that transaction and the first error stops it before the next step
   starts; the context cancels any of them, and a transaction the
   context ends, between two statements, inside one or before the
   commit, rolls back and fails with the context's error, which the
   shim answers with a 504 when the route's timeout expired
   (§route-loads) and a 499 when the client closed the request
   (§requests-and-responses). The transaction keeps its connection
   either way, so an in-memory database, which lives as long as its
   one connection, and the connection's settings outlive it. Inside a transaction the
   steps run in order because a transaction holds one connection,
   which runs one statement at a time anyway, and because SQLite rolls
   the whole transaction back when a write inside it is interrupted, as
   a cancelled context interrupts it, or fails as a full disk does,
   leaving the connection in autocommit: a step that went on beside it
   would write outside the transaction, each statement committing on
   its own. A step of a `Parallel` outside a transaction that panics
   cancels the rest as well, and once every step has joined its panic
   is raised again, carrying the step's stack, on the goroutine that
   performs the `Parallel`: it ends what a panic there would end, the
   request alone, which `net/http` recovers, and never the process.
   Inside a transaction a step runs on that goroutine already, and the
   transaction rolls back. The runner writes one
   structured log line per statement at debug level, with the text,
   the duration and the error, through the logger of `WithLogger`.
   `rt.Null[T]` marshals as the value or `null`.
6. **Effects over earlier results.** `rt.Then(e, next)` performs `e`,
   calls `next` with its result and performs the effect `next` returns,
   on the same executor, and yields that effect's result; `rt.Map(e, f)`
   performs `e` and yields `f` of its result; `rt.Atomic(e)` performs
   `e`, and everything `e` performs, in one transaction that commits
   when all of it succeeds and rolls back on the first error, which it
   returns. `rt.Pure(v)` performs nothing and yields `v`, and
   `rt.Fail(err)` performs nothing and fails with `err`: a `next`
   returns the one when nothing is left to perform and the other when
   it refuses what it read. The five are effects like the others,
   values the runner performs, and their constructors are the one way
   to build them. They nest, and `next` may return any effect: an
   `Atomic` over the `Then` of `PostCreate(c)` whose `next` returns the
   `Transaction` of the inserts that carry the new post's key writes
   the post and its tags together or not at all, and an `Atomic` over
   the `Then` of `PostGet(id)` whose `next` returns
   `rt.Fail(volt.Error(409, ...))` for a post that is no draft, and its
   update otherwise, publishes a draft and nothing else.
   - `next` is a pure function from a value to a description and `f`
     one from a value to a value (§values-and-effects): the runner
     calls them between the statements, and nothing performs inside
     them, and they write no variable they capture, which would make
     the effect perform differently each time and, inside a `Parallel`
     outside a transaction, race with itself; in a routes package the
     vet rules of `lint.md` hold them as they hold the handler, and
     `capturedwrite` reports a write to a captured variable. Neither is
     called when the effect before it fails. A panic inside either is
     the request's, as a handler's is: `net/http` ends that request
     alone, inside a `Parallel` too, which raises it again on the
     goroutine that performs it, and the transaction it ran in rolls
     back.
   - A nil is never an effect: a nil `Effect` where one is performed,
     a `Transaction`'s or a `Parallel`'s step, the `e` of a `Then`, a
     `Map` or an `Atomic` and the effect a `next` returns among them, a
     nil `next` or `f`, and an `rt.Fail` of a nil error yield
     `rt.ErrNoEffect`, which stops the effect there as any other error
     does; a `Transaction`, a `Parallel`, a `Then` and a `Map` check
     what they hold before they perform any of it.
   - A `Then` is not atomic by itself. Outside an `Atomic` and a
     `Transaction` its steps run one after another, each on its own, so
     when the effect `next` returns fails, what `e` wrote stays
     written. Inside either, every step runs on that one transaction.
   - An `Atomic` inside a `Transaction` or another `Atomic` opens no
     transaction of its own and joins the one already open, as a
     nested `Transaction` does: its failure rolls back the whole of the
     outer one, and what it wrote commits only when the outer one does.
   - A `Parallel` inside an `Atomic` does what it does inside a
     `Transaction` (rule 5): it runs its steps in order on the one
     transaction, every step joining it, and the first error stops it
     before the next step starts and rolls back everything the
     `Atomic` performed.
   - `rt.Map(e, f)` performs what an `rt.Then` over `e` does whose
     `next` returns `rt.Pure(f(v))`, and `rt.Fail(err)` what a kind
     whose `Err` is set does: `Map` is the short spelling of the
     common case, and `Fail` is the refusal a `next` of any result
     returns without building a statement it never runs. `volt.Same`
     compares spellings, so each pair is two effects there
     (§requests-and-responses).
   - An error comes back as the step reported it, unwrapped, so a
     refused write is still the `*rt.Invalid` the error handler answers
     with 409 or 422, a miss still `rt.ErrNotFound`, a 404, and an
     `rt.Fail` the error it carries, an `HTTPError` with its own status
     among them (§formats).
7. **Locals and helpers.** Every local a function of a data file
   declares, the parameters of a function literal included, is `volt`
   followed by an upper-case letter, with no underscore (`voltErr`,
   `voltRows`, `voltV`), and so is every unexported package-level name
   the data files declare: the scan function `volt<Row>Scan` of a
   model or a select's row type, a model's `volt<Model>Desc`,
   `volt<Model>Values`, `volt<Model>Apply` and `volt<Model>Cast`
   (§changesets), and the statement constants
   `volt<Model>TableSQL`, `volt<Model>ColumnsSQL`, `volt<Model>GetSQL`,
   `volt<Model>GetBy<Field>SQL`, `volt<Model>ListSQL`,
   `volt<Model>DeleteSQL` and `volt<Model><Select>SQL`. The parameter a
   key column takes in `Get`, `GetMany`, `GetBy*` and `Delete` is its
   Go field name (§names) with the leading initialism or letter lowered
   (`UserID` is `userID`), with `_` appended when that is a Go keyword,
   `ctx`, `arg` or `rt`, `nil`, `append`, `any` or `true` (the predeclared names a
   generated method uses), a name the route files' signatures take
   (`w`, `r`, `opts`, `volt`, `cs`, `c`, `body`), which a query
   route's path parameter, named after the key parameter, may not take
   (§route-paths), or of the locals' form: a column `volt_err` takes
   `voltErr_`, a column `c` takes `c_` and a column `body` takes
   `body_`. `GetMany` takes that name
   with `s` added. A select parameter is refused where a key column's
   parameter takes `_`, `arg` aside (§predicates), so no column and no
   parameter shadows a local, a helper or a predeclared name a method
   uses: key columns named `err`, `res`, `n`, `out`, `rows`, `q`, `nil`
   or `post_scan` generate methods that compile, and a table keyed on
   `c` routes as `/:c_` (§query-routes).

---

## Mapping to SQLite DDL

*Generation contract of `volt gen --sql` (`volt_schema.sql`). Pinned by
goldens that are executed on a real SQLite (`PRAGMA foreign_keys = ON`,
`foreign_key_check` after), seed INSERTs included.*

| Declared | SQLite |
|---|---|
| column without `null`, key columns included (SQLite's `PRIMARY KEY` does not imply NOT NULL) | `NOT NULL`, the rowid alias below excepted; a column that carries `null` has no nullability constraint (§columns) |
| `int` `[pk]`, the one column of the key (§columns) | exactly `INTEGER PRIMARY KEY`, without `NOT NULL` and without `DEFAULT`, which SQLite would ignore there, so `default:` on it is a check error (§default-values): the rowid alias, which SQLite assigns whenever an insert leaves it unmarked. It is the database-assigned key unless it is a foreign key; a shared-key one-to-one (`user_id int [pk, ref: users.id]`) renders the same, with its `FOREIGN KEY`, and takes its value from the referenced row (§relationships-ref). A key of a declared type stored as `int` renders the same, so that a migration's `int [pk]` folds equal to it (§the-fold) |
| `int` `[pk, autoincrement]` | exactly `INTEGER PRIMARY KEY AUTOINCREMENT`: SQLite's keyword with SQLite's meaning, an id is never reused and SQLite keeps `sqlite_sequence`. The SQLite documentation advises it only when ids must never be reused, since it costs CPU, memory, disk space and disk I/O. Any other `autoincrement` placement, a key that is a foreign key among them, is a check error (§columns) |
| `pk` on one column | `PRIMARY KEY` on the column, `NOT NULL` with it, the rowid alias above excepted |
| `pk` on several columns (composite key, §columns) | `PRIMARY KEY (a, b)` table constraint listing the `pk` columns in the table's effective column order (§partial-injection-order); members get `NOT NULL` |
| `unique` or `index` column (§columns) | `CREATE UNIQUE INDEX` or `CREATE INDEX` `idx_<table>_<column> ON <table> (<column>)` after the table, before the indexes of its `indexes` blocks; never an inline `UNIQUE`, which SQLite enforces the same way but which no `drop index` removes, only a `rebuild table` |
| `indexes` entry | `CREATE INDEX <name> ON <table> (<key>)`, `UNIQUE` for `unique`, `WHERE <expr>` for `where:` (§indexes); the name is `name:` when given, else `idx_<table>_<col>[_<col>...]` from the SQLite table name and the key columns; two indexes of the schema of one name, or an index named as a table, are a check error, since SQLite names indexes per database (§indexes) |
| relationship (`ref:`, `ref:`) | `FOREIGN KEY (<fk>) REFERENCES <table> (<columns>)` with `ON DELETE` / `ON UPDATE`; the foreign key is the left endpoint of a `ref` and the declaring column of a `ref:` (§relationships-ref) |
| one-to-one (a unique foreign key, §relationships-ref) | the uniqueness as written, the column's unique index or the composite key or index; the relationship adds nothing. A shared int key is `INTEGER PRIMARY KEY` with its `FOREIGN KEY`, the rowid alias but no database-assigned key (§columns) |
| `int`, `bool` | `INTEGER` (booleans stored 0/1; `true`/`false` in records and defaults become 1/0) |
| `float64` | `REAL` |
| `string` | `TEXT` |
| `time.Time` | `TIMESTAMP`, holding the stored form of §go-types, SQLite's datetime text in UTC (the sqlite3 driver keys its time conversion on this spelling, so code reading the column through `database/sql` itself gets a `time.Time`; Volt's own reads parse through `rt.Time` and need no conversion, and the price, text the driver cannot parse reading as the zero time, is the limit §go-types states) |
| `[]byte` | `BLOB` |
| declared `type` (§types) | the column type of its storage type; a value set (§types) carries no `CHECK`, its values are the Go constants |
| backtick expressions | verbatim; parenthesized in `DEFAULT` as SQLite requires |
| validators, `required` | nothing; the database never carries a `CHECK` (§validators) |
| `records` | multi-row `INSERT` statements after all tables, in source order; `null` becomes `NULL` |
| unknown type | check error naming the column (§columns); the generator never sees one |

Identifiers are quoted only when needed (not a plain lower-case name,
or a SQLite keyword): `order` → `"order"`, `Users` → `"Users"`,
`żółw` → `"żółw"`. The DDL carries no comment but its header: doc
comments stay in the Volt source (§doc-comments-in-generated-code).
Output opens with
`PRAGMA foreign_keys = ON;` and the `-- Code generated … DO NOT EDIT.`
marker, with the same overwrite protection as §mapping-to-go.

---

## License and Provenance

Part I's syntax descends from the DBML language as documented and
implemented in [holistics/dbml](https://github.com/holistics/dbml)
(the `@dbml/parse` reference implementation). Volt is not DBML: a
DBML file is in general not a Volt file, and the conformance corpus
(§conformance-and-the-proof-chain) records what Volt accepts, not the
upstream implementation. Like the upstream project, this specification
is licensed under the [Apache License 2.0](../LICENSE).
