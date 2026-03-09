# GSX Cheat Sheet

GSX mixes normal Go file structure with JSX-like HTML authoring.

## File Shape

```gsx
package main

import "fmt"
import shared "github.com/jlcsoftgenie/gsx/examples/shared/layouts"
```

Then define one or more components:

```gsx
component Page(title string) {
  <h1>{title}</h1>
}
```

## Components

```gsx
component UserCard(user User) {
  <li class="user-card">
    <h3>{user.Name}</h3>
  </li>
}
```

Rules:
- component names are capitalized
- parameters are typed Go parameters
- component bodies contain GSX nodes

## Elements

Normal HTML-like tags:

```gsx
<div class="card">
  <p>Hello</p>
</div>
```

Self-closing tags:

```gsx
<img src={user.AvatarURL} alt={user.Name} />
<meta charset="utf-8" />
```

## Expressions

Use `{...}` for Go expressions:

```gsx
<h1>{title}</h1>
<p>{user.Name}</p>
<span>{len(users)}</span>
```

Use expressions in attributes too:

```gsx
<a href={user.ProfileURL}>{user.Name}</a>
<input value={query} />
```

## Conditionals

```gsx
if len(users) == 0 {
  <p>No users found.</p>
} else if len(users) == 1 {
  <p>1 user found.</p>
} else {
  <p>{len(users)} users found.</p>
}
```

## Loops

```gsx
<ul>
  for _, user := range users {
    <UserCard user={user} />
  }
</ul>
```

## Component Calls

Capitalized tags call components:

```gsx
<UserCard user={user} />
<Layout title={title}>
  <p>Body</p>
</Layout>
```

Cross-package components use normal Go import aliases:

```gsx
import shared "github.com/jlcsoftgenie/gsx/examples/shared/layouts"

<shared.BaseLayout title={title}>
  <p>Hello</p>
</shared.BaseLayout>
```

## Layouts And Slots

Define slots in a layout:

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

Pass slot content from the caller:

```gsx
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
```

## Built-in GSX Tags

Fragment:

```gsx
<fragment>
  <p>One</p>
  <p>Two</p>
</fragment>
```

Named fragment for slot routing:

```gsx
<fragment slot="head">
  <meta name="description" content={description} />
</fragment>
```

Slot outlet:

```gsx
<slot />
<slot name="head" />
```

Raw trusted HTML:

```gsx
<raw html={runtime.HTML("<strong>trusted</strong>")} />
```

## Attributes

Static string:

```gsx
<div class="hero" />
```

Expression:

```gsx
<a href={url} />
```

Boolean shorthand:

```gsx
<button disabled>Save</button>
```

Boolean expression:

```gsx
<button disabled={isDisabled}>Save</button>
```

Supported:
- normal HTML attributes
- `data-*`
- `aria-*`

Not supported:
- generic attribute spread

## Comments And Doctype

```gsx
<!doctype html>
<!-- comment -->
```

## Escaping

Escaped by default:

```gsx
<p>{user.Bio}</p>
<a href={user.URL}>{user.Name}</a>
```

Only explicit trusted HTML bypasses escaping:

```gsx
<raw html={runtime.HTML("<strong>trusted</strong>")} />
```

## Whitespace

Indentation-only whitespace between tags is ignored.
Inline text is preserved.

Example:

```gsx
<p>Hello {name}</p>
```

renders with the inline spacing intact.

## Quick Rules

- `package` is required
- `import` works like Go
- `component Name(args...) { ... }` defines a component
- lowercase tags are HTML-like elements
- capitalized tags are GSX components
- `{...}` contains Go expressions
- `if` and `for` use Go syntax
- `<slot />` and `slot="name"` implement layouts
- `<raw html={runtime.HTML(...)} />` is the raw HTML escape hatch
