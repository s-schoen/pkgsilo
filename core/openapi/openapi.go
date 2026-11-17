// Package openapi describes the HTTP API in an OpenAPI 3.1 document, which a Document serves.
//
// A feature describes the routes of its api package in a Section, whose Operations name the Go types of
// their request and response bodies; the schemas of those types come from core/typeschema, like those of the
// MCP tools. app adds each Section to a Spec under the path where it mounts the routes, and Spec.Build
// returns the Document.
//
// An operation names only what's specific to it. Build adds what the shared packages of the API do: the
// envelopes of core/envelope around its bodies, the problems of core/request for its request body and
// pagination, the 401 of authn.Require unless the route is public, and the 500 of every route.
package openapi

import (
	"net/http"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"

	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
)

// version is the OpenAPI version of the document.
const version = "3.1.1"

// Names of parameter locations, schema types, formats and problem members that the document repeats.
const (
	inPath             = "path"
	inQuery            = "query"
	typeString         = "string"
	typeInteger        = "integer"
	formatURIReference = "uri-reference"
	memberType         = "type"
	memberStatus       = "status"
)

// Info is the metadata of the API.
type Info struct {
	Title       string
	Version     string
	Description string
}

// SecurityScheme is a way to authenticate the routes that aren't public: a credential in a header or a cookie.
type SecurityScheme struct {
	// Name identifies the scheme in the document.
	Name string
	// Header is the name of the header that holds the credential, or empty for a cookie.
	Header string
	// Cookie is the name of the cookie that holds the credential, or empty for a header.
	Cookie      string
	Description string
}

// Section is the description of the routes of a feature's api package, relative to the path where it's
// mounted. It becomes a tag of the document that groups its operations.
type Section struct {
	// Name is the tag's name, such as Identities.
	Name        string
	Description string
	Operations  []Operation
}

// Operation describes a route: a method and a path pattern.
type Operation struct {
	// Method is the HTTP method, such as http.MethodGet.
	Method string
	// Pattern is the route's chi pattern relative to its mount path, such as / or /{id}, without regular
	// expressions. A trailing slash is dropped from the document's path.
	Pattern string
	// ID is the operation's unique name in camel case, such as getIdentity.
	ID          string
	Summary     string
	Description string
	// Parameters are the path parameters of the full path, the mount path's included, and the query
	// parameters. A Collection response adds those of the pagination.
	Parameters []Parameter
	// Body is the request body, nil for none.
	Body *Body
	// Responses are the successful responses.
	Responses []Response
	// Problems are the problems that the route answers with, besides those that Build adds.
	Problems []envelope.ProblemType
	// Public tells that the route needs no authentication, in a section whose other routes do, such as a
	// login. A section whose routes are all public is mounted with MountPublic instead.
	Public bool
}

// Parameter is a path or query parameter.
type Parameter struct {
	Name string
	// In is path or query.
	In          string
	Description string
	// Required is implied for a path parameter.
	Required bool
	Schema   *jsonschema.Schema
}

// PathID returns the path parameter name that holds the UUID of a resource. A value that isn't a UUID is
// answered like an unknown one (see request.UUIDParam), which the operation lists in its Problems.
func PathID(name, description string) Parameter {
	return Parameter{
		Name: name, In: inPath, Description: description, Required: true,
		Schema: &jsonschema.Schema{Type: typeString, Format: "uuid"},
	}
}

// Body is a request body: a JSON object whose schema is that of a Go type.
type Body struct {
	mediaType   string
	typ         reflect.Type
	description string
}

// JSONBody returns an application/json body of the type T, decoded by request.DecodeJSON.
func JSONBody[T any](description string) *Body {
	return &Body{mediaType: "application/json", typ: reflect.TypeFor[T](), description: description}
}

// MergePatchBody returns a JSON Merge Patch body of the type T, decoded by request.DecodeMergePatch.
func MergePatchBody[T any](description string) *Body {
	return &Body{mediaType: request.MergePatchMediaType, typ: reflect.TypeFor[T](), description: description}
}

// shape is the form of a successful response's body.
type shape int

const (
	// noBody is a response without a body.
	noBody shape = iota
	// plain is a body without an envelope, written by envelope.WriteJSON.
	plain
	// resource is the envelope of envelope.WriteResource and envelope.WriteCreated.
	resource
	// collection is the envelope of envelope.WriteCollection, which is paginated.
	collection
)

// Response is a successful response.
type Response struct {
	status      int
	description string
	shape       shape
	typ         reflect.Type
	location    bool
}

// Resource returns the 200 response of envelope.WriteResource, whose data is a T.
func Resource[T any](description string) Response {
	return Response{status: http.StatusOK, description: description, shape: resource, typ: reflect.TypeFor[T]()}
}

// Created returns the 201 response of envelope.WriteCreated, whose data is a T and whose Location header
// is the new resource's URL.
func Created[T any](description string) Response {
	return Response{
		status: http.StatusCreated, description: description, shape: resource, typ: reflect.TypeFor[T](),
		location: true,
	}
}

// Collection returns the 200 response of envelope.WriteCollection with a page of T. It adds the query
// parameters of request.ParsePagination to the operation, and the problem it answers them with.
func Collection[T any](description string) Response {
	return Response{status: http.StatusOK, description: description, shape: collection, typ: reflect.TypeFor[T]()}
}

// NoContent returns a 204 response.
func NoContent(description string) Response {
	return Response{status: http.StatusNoContent, description: description, shape: noBody}
}

// JSON returns a response with the given status whose body is a T without an envelope, written by
// envelope.WriteJSON.
func JSON[T any](status int, description string) Response {
	return Response{status: status, description: description, shape: plain, typ: reflect.TypeFor[T]()}
}

// Spec collects the sections of the API for Build.
type Spec struct {
	Info Info
	// Security are the ways to authenticate the routes that aren't public, any one of which is enough.
	Security []SecurityScheme
	// CrossOriginProtected tells that middleware.CrossOriginProtection guards every route, so that every POST,
	// PUT, PATCH and DELETE may answer with its problem.
	CrossOriginProtected bool
	mounts               []mount
}

// mount is a section at the path where its routes are mounted.
type mount struct {
	path    string
	section Section
	public  bool
}

// Mount adds section, whose routes are mounted at path and need authentication.
func (s *Spec) Mount(path string, section Section) {
	s.mounts = append(s.mounts, mount{path: path, section: section})
}

// MountPublic adds section, whose routes are mounted at path and need no authentication.
func (s *Spec) MountPublic(path string, section Section) {
	s.mounts = append(s.mounts, mount{path: path, section: section, public: true})
}
