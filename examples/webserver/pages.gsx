package main

import shared "github.com/jlcsoftgenie/gsx/examples/shared/layouts"

component SiteNav() {
  <nav>
    <a href="/">Home</a>
    <span> | </span>
    <a href="/users">Users</a>
    <span> | </span>
    <a href="/healthz">Health</a>
  </nav>
}

component UserRow(user User) {
  <tr>
    <td>{user.Name}</td>
    <td>{user.Email}</td>
    <td>{user.Role}</td>
  </tr>
}

component UserTable(users []User) {
  <table>
    <thead>
      <tr>
        <th>Name</th>
        <th>Email</th>
        <th>Role</th>
      </tr>
    </thead>
    <tbody>
      for _, user := range users {
        <UserRow user={user} />
      }
    </tbody>
  </table>
}

component HomePage(title string, description string, users []User) {
  <shared.Panel title={title}>
    <fragment slot="head">
      <meta name="description" content={description} />
    </fragment>
    <SiteNav />
    <section>
      <h1>{title}</h1>
      <p>{description}</p>
      <p>Total users: {len(users)}</p>
      <p>
        <a href="/users">Browse the user directory</a>
      </p>
    </section>
  </shared.Panel>
}

component UsersPage(title string, users []User) {
  <shared.Panel title={title}>
    <fragment slot="head">
      <meta
        name="description"
        content="Example users page rendered in a standard webserver."
      />
    </fragment>
    <SiteNav />
    <section>
      <h1>{title}</h1>
      if len(users) == 0 {
        <p>No users available.</p>
      }
      else {
        <UserTable users={users} />
      }
    </section>
  </shared.Panel>
}
