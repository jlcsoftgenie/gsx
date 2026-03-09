package main

component AppLayout(title string) {
  <!doctype html>
  <html>
    <head>
      <meta charset="utf-8" />
      <title>{title}</title>
      <meta name="viewport" content="width=device-width, initial-scale=1" />
    </head>
    <body>
      <slot />
    </body>
  </html>
}

component UserCard(user User) {
  <li class="user-card">
    <img src={user.AvatarURL} alt={user.Name} />
    {user.Name}
    <div class="content">
      <h3>{user.Name}</h3>
      <p>{user.Email}</p>
    </div>
  </li>
}

component UsersPage(title string, users []User) {
  <AppLayout title={title}>
    <main class="container">
      <h1>{title}</h1>
      if len(users) == 0 {
        <p>No users found.</p>
      }
      else {
        <ul class="users">
          for _, user := range users {
            <UserCard user={user} />
          }
        </ul>
      }
    </main>
  </AppLayout>
}
