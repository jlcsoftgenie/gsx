package layouts

component Shell(title string) {
  <!doctype html>
  <html>
    <head>
      <meta charset="utf-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1" />
      <title>{title}</title>
      <slot name="head" />
    </head>
    <body>
      <header class="site-header">
        <strong>Shared Layouts</strong>
      </header>
      <main>
        <slot />
      </main>
    </body>
  </html>
}

component Panel(title string) {
  <Shell title={title}>
    <slot name="head" />
    <section class="panel-shell">
      <slot />
    </section>
  </Shell>
}
