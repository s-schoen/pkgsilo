# Authorization in a feature

A service enforces authorization with the `authz.Authorizer` it's given (`core/authz`, whose package doc describes the model): `For(ctx, actor)` once per request, then `Can(action, resource)` for each object.
Everything that no policy permits is denied.
`app.Bootstrap` passes `authz.AllowAll()`; every actor value goes through the checks.

An error from `For` or `Can` is wrapped with `%w` and becomes a logged 500; only `authz.ErrDenied` and `*authz.FieldsDeniedError` are denials.
The API answers an `*authz.FieldsDeniedError` with `request.FieldsDeniedProblem` and `authz.ErrDenied` with a 403.

## Actions (`actions.go`)

`authz.DeclareType` declares the feature's resource type and its actions, named `<type>:<verb>` after the type in lower case:

- `<type>:read`, in `authz.ActionGroupRead`, and `<type>:delete`.
- The field actions, derived from the `authz` tags of the types in its `authz.TypeSpec`:
  - `<type>:create:<field>` and `<type>:update:<field>`, grouped under `<type>:create` and `<type>:update`, for every member of `TypeSpec.Create` and `TypeSpec.Update`, each tagged `authz:"<field>"` with the name of its request member.
    `TypeSpec.Patch` holds the same members as pointers.
  - `<type>:read:<field>` for every field of the domain model, `TypeSpec.Model`, but the ID, grouped under `read` and `<type>:readFields`.
- Further actions, declared with `authz.NewAction`, those that read in `authz.ActionGroupRead`.

`Actions()` exports them: `Actions().Declaration()` is the `authz.Declaration` that `app/app.go` adds to `authz.Schema`, so that saving a policy with an undeclared action fails validation, and `Actions().ReadFields().Fields()` returns the read field names for the handler test.

**Tags are a public contract.**
The field actions are named after the `authz` tags, so renaming a tag renames an action that stored policies may name.
`app/testdata/schema.cedarschema` pins the schema; a deliberate change rewrites it with `go test ./app -run TestSchema -update`.

## One object

An object the actor can't read gets the same domain error as a missing one, so that both answer with the same 404.

## Reads and allowed actions

The domain model holds every field but `ID` as an `authz.Field` tagged `authz:"<field>"` with the name of its response member, and a member `AllowedActions authz.AllowedActions`, which `DeclareType` requires.
The service builds it from its record with `authz.NewMapper[<Name>Record, <Model>](actions)`, a package variable named `to<Model>`, on the `authz.Access` that `actions.Access(decider, resource)` returned: it reveals each field by its tag and fills `AllowedActions`.
`NewMapper` fails the package's initialization if a member of the model but `AllowedActions` has no member of the record with the same name and type, so that no field can be forgotten.

`AllowedActions` lists the actions that the actor may do to the object, so that a client offers only those: every action of `actions.Allowable()`, which is `<type>:delete`, `<type>:update:<field>` and the further actions, read ones too, such as `group:readMembers`.
It holds no `<type>:read`, `<type>:read:<field>` (the object and its visible fields tell those) and no `<type>:create:<field>` (no existing object has them).
`Access` enforces the field reads with `Can` and asks about the actions with `Decider.Hint`, which decides alike but counts as a hint in the decisions metric.
The list is advisory: the service still checks every request.

The service gets the access of one object after its read check, of a list only for the rows on the page, and of a write before writing, as the object was before it.
A write's response is masked like a read.
A `POST` gets its access from `AccessIf`, which checks the object's read action first, so that an object the actor can't read returns only its ID, without `AllowedActions`.
A model that doesn't map member by member onto its record is built by hand from `Access.Fields()` and `Access.AllowedActions()`.

`feature/identity/service` is the reference; its tests expect the hints with `servicetest.ExpectAllowedActions`.

## Writes

Every check runs before anything is written, and an existing object's read check comes first.

- `POST` checks the field action of every member it sends.
  An optional input field is a pointer, and a `POST` that leaves it out checks only the fields it sends, with `FieldActions.Only(authz.SetFields(in)...)`.
- `PUT` checks every settable field, even one that doesn't change.
- `PATCH` checks only the fields it sets, with `FieldActions.Only(authz.SetFields(patch)...)`.
  A `PATCH` that sets no field writes nothing and returns the object.
- `DELETE` checks `<type>:delete`.

A handler test checks that each settable request member has a field action.

## Child objects and references

An endpoint on a child of an object, such as a group's members, checks `read` on the parent first and answers a denial with the parent's 404, then checks an action on the parent such as `group:addMember`.

A request that refers to another feature's object checks that feature's read action on it, exported by its service (`identityservice.Actions().Read()`), and answers a denial like a missing object, so that it can't reveal whether a hidden object exists.

## Lists with per-row rules

These replace `limit+1`, because a filtered page would be short and reveal hidden rows.
The repository has `ListAll` instead of `List`.
The service loads the rows, checks every one with `authz.Filter` and cuts out the page with `pagination.FromAll`, so that only the last page can be short.

## Group members and owned resources

- A resource that can be a group member passes its groups in `authz.Resource.Groups`, loaded in bulk with one query per list.
  Without them, a `forbid` on the group silently allows.
- A resource that belongs to an identity, such as an API key, sets `Owned` in its `authz.TypeSpec` and passes its owner, with the owner's groups, in `authz.Resource.Owner`, so that policies can use `resource.owner`.
  The owner is the only attribute: secrets stay out of the schema.
  The service loads the owner with `authz.OwnerLoader.ReadableOwner`, which reports an identity that the actor can't read like a missing one, and translates `authz.ErrOwnerNotFound` to its own identity-not-found error.
- A type whose identity owns at most one resource, such as a password, also sets `OnePerOwner`: its resource's ID is the owner's.
  A create checks `TypeActions.NewResource(owner)`, which is the owner's resource for a one-per-owner type and a fresh ID otherwise; the service stores the new resource under that ID.

## No filters on hidden data

Never add a filter or search parameter on a field the caller can't read: it turns a timing side channel into "does this exact value exist?".
The package doc of `core/authz` lists this and the other accepted risks.
