# Escaping And Raw HTML

GSX is safe-by-default.

## Default escaping

These are escaped automatically:
- text expressions: `{user.Name}`
- dynamic attribute values: `href={url}`
- boolean attributes with dynamic values when they render as strings

Examples:

```gsx
<p>{user.Bio}</p>
<a href={user.URL}>{user.Name}</a>
```

Rendered output escapes HTML-sensitive characters such as:
- `&`
- `<`
- `>`
- `"`
- `'`

## Trusted raw HTML

Raw HTML is explicit and opt-in:

```gsx
import "github.com/jlcsoftgenie/gsx/runtime"

component Trusted(content runtime.HTML) {
  <raw html={content} />
}
```

Rules:
- `runtime.HTML` is the trusted raw type
- raw content is only emitted through `<raw html={...} />`
- normal `{expr}` rendering does not bypass escaping

That distinction prevents accidental unsafe insertion from plain string expressions.

## Attribute safety

Dynamic attributes are emitted with quoted values and escaped content.
Boolean HTML attributes are emitted with the attribute name only when their value is `true`.

Examples:

```gsx
<input disabled={form.ReadOnly} />
<a href={link}>View</a>
```

## Security guidance

Use raw HTML only when:
- the content is fully trusted
- it was sanitized before reaching the template
- you have a specific rendering need that escaped output cannot satisfy

Prefer normal expressions everywhere else.
