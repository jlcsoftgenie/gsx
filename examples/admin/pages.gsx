package main

component MetricCard(metric Metric) {
  <article class="metric-card">
    <h2>{metric.Value}</h2>
    <p>{metric.Label}</p>
  </article>
}

component DashboardLayout(title string) {
  <!doctype html>
  <html>
    <head>
      <meta charset="utf-8" />
      <title>{title}</title>
      <slot name="head" />
    </head>
    <body>
      <aside class="sidebar">
        <nav>
          <a href="/">Overview</a>
          <a href="/users">Users</a>
          <a href="/jobs">Jobs</a>
        </nav>
      </aside>
      <section class="content">
        <slot />
      </section>
    </body>
  </html>
}

component DashboardPage(data DashboardData) {
  <DashboardLayout title={data.Title}>
    <fragment slot="head">
      <meta name="description" content="Example admin dashboard rendered with GSX" />
    </fragment>
    <header>
      <h1>{data.Title}</h1>
      <form method="post" action="/filters">
        <label for="range">Time range</label>
        <select id="range" name="range">
          <option value="24h">Last 24 hours</option>
          <option value="7d">Last 7 days</option>
          <option value="30d">Last 30 days</option>
        </select>
        <button type="submit">Apply</button>
      </form>
    </header>
    <section class="metrics-grid">
      for _, metric := range data.Metrics {
        <MetricCard metric={metric} />
      }
    </section>
  </DashboardLayout>
}
