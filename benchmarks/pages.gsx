package benchmarks

component BenchLayout(title string) {
  <!doctype html>
  <html>
    <head>
      <meta charset="utf-8" />
      <title>{title}</title>
    </head>
    <body>
      <slot />
    </body>
  </html>
}

component BenchUserRow(user User) {
  <li class="user-row">
    <strong>{user.Name}</strong>
    <span>{user.Email}</span>
  </li>
}

component BenchSimple(title string) {
  <BenchLayout title={title}>
    <main>
      <h1>{title}</h1>
      <p>Fast, compiled HTML rendering.</p>
    </main>
  </BenchLayout>
}

component BenchBaseLayout(title string) {
  <!doctype html>
  <html>
    <head>
      <meta charset="utf-8" />
      <title>{title}</title>
      <slot name="head" />
    </head>
    <body>
      <slot />
    </body>
  </html>
}

component BenchShellLayout(title string) {
  <BenchBaseLayout title={title}>
    <slot name="head" />
    <section class="shell">
      <slot />
    </section>
  </BenchBaseLayout>
}

component BenchList(title string, users []User) {
  <BenchLayout title={title}>
    <main>
      <h1>{title}</h1>
      <ul>
        for _, user := range users {
          <BenchUserRow user={user} />
        }
      </ul>
    </main>
  </BenchLayout>
}

component BenchNestedLayouts(title string, users []User) {
  <BenchShellLayout title={title}>
    <fragment slot="head">
      <meta name="description" content={title} />
    </fragment>
    <main>
      <h1>{title}</h1>
      <ul>
        for _, user := range users {
          <BenchUserRow user={user} />
        }
      </ul>
    </main>
  </BenchShellLayout>
}
