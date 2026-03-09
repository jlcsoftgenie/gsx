# Migrating From `html/template`

GSX is not a drop-in replacement for `html/template`, but the migration path is straightforward when you want compiled components and layouts.

## Mental model shift

`html/template`:
- templates are strings parsed at runtime or startup
- layouting is often done with `define` and `template`
- dot-driven data access is central

GSX:
- templates are source files compiled ahead of time
- components have typed Go parameters
- layouts use default and named slots
- expressions are plain Go snippets

## Typical migration steps

1. Identify repeated page shells and convert them into layout components.
2. Replace large partial templates with small typed components.
3. Move template input data from anonymous maps into Go structs.
4. Compile templates into generated `.go` files.
5. Replace `template.Execute` with `Render<Component>` calls.

## Example

`html/template` style:

```go
tmpl.Execute(w, struct {
    Title string
    Users []User
}{Title: title, Users: users})
```

GSX style:

```go
if err := RenderUsersPage(w, title, users); err != nil {
    return err
}
```

## Escaping differences

Both systems escape by default, but GSX makes raw HTML explicit through the `runtime.HTML` type and `<raw html={...} />` node.

## Layout differences

`html/template` often uses named templates and template inclusion.
GSX uses components and slots:
- `<slot />` for the default body
- `<slot name="head" />` for named sections
- wrapper components can forward slots to nested layouts

## Recommended adoption strategy

Start with one vertical slice:
- one layout
- one page
- two or three leaf components

That keeps the migration bounded while letting you evaluate generated output, render speed, and team ergonomics.
