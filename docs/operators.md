# Operating pkgsilo

pkgsilo reads its configuration from the environment; `.env.example` lists every variable with its meaning.

## First start

On its first start, against a database without identities, pkgsilo creates:

- the identity `admin`, with no password;
- the group `administrators`, with `admin` as its member, and a policy that grants the group everything;
- an API key of `admin` that never expires, the bootstrap key.

It logs the bootstrap key once, at warn level, as `api_key` in the message `created identity admin with an api
key that never expires`. The key can't be retrieved later.

There is no default password and no password in the environment. To log in as `admin` with a password, reset
its password with the bootstrap key, here in bash:

```bash
export PKGSILO_URL=https://pkgsilo.example.com
export PKGSILO_KEY=psk_...   # the bootstrap key from the log

# The ID of admin, the only identity so far.
ADMIN_ID=$(curl -s -H "X-API-Key: $PKGSILO_KEY" "$PKGSILO_URL/api/v1/identities" | jq -r '.data[0].id')

# Prompts for the password and pipes it through the printf builtin, so that it stays out of the shell
# history and of every process's arguments.
read -rsp 'Password: ' PASSWORD; echo
printf '%s' "$PASSWORD" | jq -Rs '{newPassword: .}' |
  curl -s -X PUT -H "X-API-Key: $PKGSILO_KEY" -H "Content-Type: application/json" --data-binary @- \
    "$PKGSILO_URL/api/v1/identities/$ADMIN_ID/password"
unset PASSWORD
```

The `administrators` policy lets its members reset any password, their own included, without the current
one. Afterwards, create an API key of your own and delete the bootstrap key.

## Passwords

- An identity has at most one password, which an administrator sets with
  `PUT /api/v1/identities/{identityID}/password`. Nobody can give themselves a first password.
- The seeded, editable policy `manage-own-password` lets every identity see when its own password was set and
  change it with `PUT`, giving the current one in `currentPassword`.
- `DELETE` on the same path removes a password; the identity then only uses its API keys.
- Passwords are stored as Argon2id hashes, each of which takes 19 MiB of memory. At most as many hashes run
  at once as Go uses cores (`GOMAXPROCS`).
- No MCP tool sets a password, so that plaintext passwords never pass through a model's context.
