package main

component BaseLayout(title string) {
  <!doctype html>
  <html>
    <head>
      <meta charset="utf-8" />
      <title>{title}</title>
      <slot name="head" />
    </head>
    <body>
      <header>
        <slot name="header" />
      </header>
      <main>
        <slot />
      </main>
      <footer>
        <slot name="footer" />
      </footer>
    </body>
  </html>
}

component AuthLayout(title string) {
  <BaseLayout title={title}>
    <slot name="head" />
    <div slot="header" class="topbar">
      <strong>Secure Area</strong>
    </div>
    <section class="auth-shell">
      <slot />
    </section>
    <slot name="footer" />
  </BaseLayout>
}

component ProfilePage(data ProfileData) {
  <AuthLayout title={data.Title}>
    <fragment slot="head">
      <meta name="description" content={data.Description} />
    </fragment>
    <section>
      <h1>{data.Title}</h1>
      <p>{data.Description}</p>
      <dl>
        <dt>Email</dt>
        <dd>{data.Email}</dd>
        <dt>Role</dt>
        <dd>{data.Role}</dd>
      </dl>
    </section>
    <div slot="footer">
      <small>Signed in as {data.Email}</small>
    </div>
  </AuthLayout>
}
