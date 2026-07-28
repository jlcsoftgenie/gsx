# GSX Syntax

GSX files use the `.gsx` extension and are compiled ahead of time into `.go` files.

## File structure

Each file starts with a package clause and optional Go imports:

```gsx
package pages

import "fmt"
import model "example.com/app/model"

component Page(title string) {
  <h1>{title}</h1>
}
```

## Components

A component is a named render unit with typed Go parameters.

```gsx
component UserCard(user User) {
  <li class="user-card">
    <h3>{user.Name}</h3>
  </li>
}
```

Public generated API:
- `RenderUserCard(w io.Writer, user User) error`
- `RenderUserCardBuffer(buf *bytes.Buffer, user User) error`
- `RenderUserCardWithSlots(w io.Writer, slots GSXUserCardSlots, user User) error`
- `RenderUserCardBufferWithSlots(buf *bytes.Buffer, slots GSXUserCardSlots, user User) error`

`Render...Buffer` is an opt-in helper for callers that need the complete HTML in a
`bytes.Buffer` before using it. It pre-grows that buffer with a compile-time
size hint. Streaming `Render...` functions remain the normal choice for HTTP
responses.

Internal generated API:
- `renderUserCard(w io.Writer, slots GSXUserCardSlots, user User) error`

## Nodes

Supported node kinds inside component bodies:
- HTML-like elements: `<div class="a">...</div>`
- self-closing elements: `<img src={url} />`
- component calls: `<Layout title={title}>...</Layout>`
- local declarations: `count := len(users)`, `var first *User`, `const label = "Users"`
- expressions: `{expr}`
- conditionals: `if cond { ... } else if other { ... } else { ... }`
- loops: `for _, item := range items { ... }`
- comments: `<!-- comment -->`
- doctype: `<!doctype html>`
- fragments: `<fragment>...</fragment>`
- slot outlets in layout components: `<slot />`, `<slot name="head" />`
- explicit trusted HTML: `<raw html={trusted} />`

## Slots and layouts

Slots are first-class. A component becomes a layout when it renders one or more `<slot>` outlets.

```gsx
component BaseLayout(title string) {
  <!doctype html>
  <html>
    <head>
      <title>{title}</title>
      <slot name="head" />
    </head>
    <body>
      <header><slot name="header" /></header>
      <main><slot /></main>
      <footer><slot name="footer" /></footer>
    </body>
  </html>
}
```

Callers pass default slot content as normal children. Named slots are attached via a `slot` attribute on direct child nodes:

```gsx
component HomePage(title string, description string) {
  <BaseLayout title={title}>
    <fragment slot="head">
      <meta name="description" content={description} />
    </fragment>

    <div slot="header">
      <h1>{title}</h1>
    </div>

    <section>
      <p>{description}</p>
    </section>

    <div slot="footer">
      <small>Footer content</small>
    </div>
  </BaseLayout>
}
```

The compiler strips the `slot` attribute from rendered HTML when it is used as slot routing metadata on direct component children.

## Cross-package components

GSX components can be imported through normal Go imports and referenced with the import alias.

```gsx
package pages

import shared "example.com/app/shared/layouts"

component HomePage(title string, description string) {
  <shared.Panel title={title}>
    <fragment slot="head">
      <meta name="description" content={description} />
    </fragment>

    <p>{description}</p>
  </shared.Panel>
}
```

Rules:
- imported GSX packages must be discoverable during `gsx generate` and `gsx check`
- default aliasing follows the Go import path base name
- explicit import aliases are recommended when the package name differs from the path base
- external component calls compile against `Render<Component>WithSlots` and `GSX<Component>Slots`

## Escaping and raw HTML

Text and attribute values are escaped by default.

```gsx
<p>{user.Bio}</p>
<a href={user.URL}>{user.Name}</a>
```

Trusted raw HTML is explicit:

```gsx
<raw html={runtime.HTML("<strong>trusted</strong>")} />
```

`runtime.HTML` is the only raw HTML type recognized by generated output helpers.

## Local declarations

GSX supports constrained Go declaration statements inside component bodies.

Supported forms:
- short declarations: `count := len(users)`
- `var` declarations: `var first *User`
- `const` declarations: `const emptyLabel = "No users"`

Example:

```gsx
component UsersPage(users []User) {
  count := len(users)
  const emptyLabel = "No users found."

  if count == 0 {
    <p>{emptyLabel}</p>
  } else {
    <p>{count} users found.</p>
  }
}
```

Rules:
- declarations must be standalone statement lines
- declarations follow normal Go block scoping
- later expressions, `if`, `for`, and attributes can use declared names
- reassignment and arbitrary statements are intentionally not supported in template bodies

## Whitespace rules

GSX preserves authored text, with one deliberate normalization to make formatted templates render predictably:
- text nodes containing only spaces, tabs, and at least one newline are ignored
- other text nodes are preserved exactly

This means indentation between tags does not leak into output, while inline spaces remain stable.

## Attribute rules

Supported attribute forms:
- static string: `class="hero"`
- expression: `href={url}`
- boolean attribute shorthand: `disabled`
- boolean expression on boolean attributes: `disabled={isDisabled}`
- `data-*` and `aria-*` attributes are valid

The current implementation intentionally does not support generic attribute spread because it adds hot-path overhead and weakens generated output predictability.

## Grammar summary

```ebnf
File            = PackageDecl ImportDecl* ComponentDecl* EOF .
PackageDecl     = "package" Ident .
ImportDecl      = "import" ImportSpec | "import" "(" ImportSpec* ")" .
ImportSpec      = [Ident | "."] StringLit .
ComponentDecl   = "component" Ident "(" ParamList? ")" TemplateBlock .
ParamList       = Param { "," Param } .
Param           = Ident TypeExpr .
TemplateBlock   = "{" Node* "}" .
Node            = Element | Doctype | Comment | DeclStmt | Expr | IfStmt | ForStmt | Text .
Element         = "<" Name Attr* ("/>" | ">" Node* "</" Name ">") .
Name            = Ident { "." Ident } | dashed-name .
Attr            = Name ["=" (StringLit | Expr)] .
DeclStmt        = ShortDecl | VarDecl | ConstDecl .
ShortDecl       = IdentList ":=" GoExpr .
VarDecl         = "var" GoDeclLine .
ConstDecl       = "const" GoDeclLine .
Expr            = "{" GoExpr "}" .
IfStmt          = "if" GoExpr TemplateBlock { "else" "if" GoExpr TemplateBlock } [ "else" TemplateBlock ] .
ForStmt         = "for" GoHeader TemplateBlock .
```

`GoExpr`, `GoHeader`, `GoDeclLine`, and `TypeExpr` are captured as balanced Go snippets and preserved verbatim in generated code.
