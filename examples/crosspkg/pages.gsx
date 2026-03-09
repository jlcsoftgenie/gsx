package main

import shared "github.com/jlcsoftgenie/gsx/examples/shared/layouts"

component HomePage(title string, description string) {
  <shared.Panel title={title}>
    <fragment slot="head">
      <meta name="description" content={description} />
    </fragment>
    <article>
      <h1>{title}</h1>
      <p>{description}</p>
    </article>
  </shared.Panel>
}
