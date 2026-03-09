# Architecture

GSX is built around ahead-of-time code generation.

Pipeline:
1. `internal/lexer` tokenizes GSX source.
2. `internal/parser` builds a typed AST while retaining source spans.
3. `internal/compiler` builds a multi-package registry, performs semantic validation, and resolves imported components by Go import path.
4. `internal/codegen` emits deterministic Go render functions.
5. `runtime` provides the minimal escaping and write helpers used by generated code.

Key decisions:
- Components compile to direct `io.Writer` calls rather than runtime node trees.
- Layout slots compile to generated slot structs instead of maps to avoid lookup overhead.
- Cross-package component calls compile against exported `Render<Component>WithSlots` functions and `GSX<Component>Slots` types.
- Expressions are preserved as Go source snippets and embedded verbatim in generated code.
- The runtime is intentionally small; complexity lives in generated code where Go can inline aggressively.
- Whitespace is normalized only for indentation-only nodes so formatted templates do not emit noisy text.
- Wrapper components that only forward `<slot>` content reuse parent slot functions directly instead of allocating new closures for those paths.
