import type { LinkProps } from "react-aria-components/Link";

// A route of the app, as React Aria's links take it in href: a path of the route tree, such as
// /identities/$id, which src/app/root-layout.tsx types.
type Href = NonNullable<LinkProps["href"]>;

// The names of a route's path params: "id" for /identities/$id.
type PathParams<TPath extends string> = TPath extends `${string}$${infer Param}/${infer Rest}`
  ? Param | PathParams<`/${Rest}`>
  : TPath extends `${string}$${infer Param}`
    ? Param
    : never;

// The href of a route with path params, for a React Aria link: routeHref("/identities/$id",
// { id }) is /identities/<id>. The type check checks the route and its params as it checks
// <Link to params>. The router matches the filled-in path as it does a typed one, but the
// RouterConfig's href only knows the routes' patterns, so the result is cast to it.
export function routeHref<const TPath extends Href>(
  path: TPath,
  params: Record<PathParams<TPath>, string>,
): Href {
  const values: Record<string, string> = params;
  return path.replaceAll(/\$(\w+)/g, (_, name: string) =>
    encodeURIComponent(values[name] ?? ""),
  ) as Href;
}
