package main

import "strconv"

component Shell(title string) {
  <!doctype html>
  <html>
    <head>
      <meta charset="utf-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1" />
      <title>{title}</title>
      <script src="https://unpkg.com/htmx.org@1.9.12"></script>
    </head>
    <body>
      <slot />
    </body>
  </html>
}

component UserRow(user User) {
  <tr id={"user-" + strconv.Itoa(user.ID)}>
    <td>{user.Name}</td>
    <td>{user.Email}</td>
  </tr>
}

component UsersTable(users []User) {
  <table class="users-table">
    <thead>
      <tr>
        <th>Name</th>
        <th>Email</th>
      </tr>
    </thead>
    <tbody>
      for _, user := range users {
        <UserRow user={user} />
      }
    </tbody>
  </table>
}

component UsersPage(users []User) {
  <Shell title="HTMX Users">
    <main>
      <h1>Users</h1>
      <div hx-get="/users" hx-trigger="load" hx-swap="innerHTML">
        <UsersTable users={users} />
      </div>
    </main>
  </Shell>
}
