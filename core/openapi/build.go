package openapi

import (
	"cmp"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"pkgsilo/core/envelope"
	"pkgsilo/core/middleware"
	"pkgsilo/core/request"
	"pkgsilo/core/typeschema"
)

// componentsPath is the prefix of a reference to a schema of the document's components.
const componentsPath = "#/components/schemas/"

// Names of the schemas that Build adds to the components.
const (
	problemSchemaName         = "Problem"
	validationErrorSchemaName = "ValidationError"
	paginationSchemaName      = "Pagination"
)

var (
	// componentName matches the names that OpenAPI allows for a component.
	componentName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	// templateParameter matches a parameter of a path template, such as {id}.
	templateParameter = regexp.MustCompile(`\{([^}]*)\}`)
	// methods are the HTTP methods that an operation may have.
	methods = []string{
		http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete, http.MethodPatch, http.MethodHead,
		http.MethodOptions,
	}
	// unsafeMethods are the methods that middleware.CrossOriginProtection checks.
	unsafeMethods = []string{http.MethodPut, http.MethodPost, http.MethodDelete, http.MethodPatch}

	errInvalidSpec = errors.New("invalid OpenAPI spec")
)

// Build returns the document of the sections that s mounts. It fails for an operation that it can't
// describe, such as one without an ID or whose path parameters don't match its path, or for a body type
// that core/typeschema rejects: bugs of the feature, found when the API is assembled.
func (s *Spec) Build() (*Document, error) {
	b := &builder{
		spec: s,
		doc: documentObject{
			OpenAPI: version,
			Info:    infoObject{Title: s.Info.Title, Version: s.Info.Version, Description: s.Info.Description},
			Paths:   pathsObject{},
			Components: componentsObject{Schemas: map[string]*jsonschema.Schema{
				problemSchemaName: problemSchema(),
			}},
		},
		components:   map[string]component{},
		operationIDs: map[string]bool{},
		tags:         map[string]bool{},
	}
	validationError := reflect.TypeFor[envelope.ValidationError]()
	if err := b.addComponent(validationErrorSchemaName, validationError, typeschema.Output); err != nil {
		return nil, err
	}
	for _, m := range s.mounts {
		if err := b.addSection(m); err != nil {
			return nil, err
		}
	}

	body, err := json.Marshal(b.doc, json.Deterministic(true), jsontext.WithIndent("  "))
	if err != nil {
		return nil, fmt.Errorf("encode OpenAPI document: %w", err)
	}

	return newDocument(body, b.routes), nil
}

// builder builds the document of a Spec.
type builder struct {
	spec *Spec
	doc  documentObject
	// components holds the types of the schemas of the document's components by name, but for the Problem.
	components   map[string]component
	operationIDs map[string]bool
	tags         map[string]bool
	routes       []Route
}

// component is the type that a schema of the document's components describes, in a direction.
type component struct {
	typ reflect.Type
	dir typeschema.Direction
}

func (b *builder) addSection(m mount) error {
	if m.section.Name == "" || b.tags[m.section.Name] {
		return fmt.Errorf("%w: section at %s: name %q is empty or taken", errInvalidSpec, m.path, m.section.Name)
	}
	b.tags[m.section.Name] = true
	b.doc.Tags = append(b.doc.Tags, tagObject{Name: m.section.Name, Description: m.section.Description})
	if !m.public {
		if err := b.secure(); err != nil {
			return err
		}
	}

	for _, op := range m.section.Operations {
		if err := b.addOperation(m, op); err != nil {
			return fmt.Errorf("%w: operation %s %s%s: %w", errInvalidSpec, op.Method, m.path, op.Pattern, err)
		}
	}

	return nil
}

// secure adds the security scheme of the spec to the document, which every operation needs unless it
// overrides it.
func (b *builder) secure() error {
	if b.doc.Components.SecuritySchemes != nil {
		return nil
	}
	scheme := b.spec.Security
	if scheme.Name == "" || scheme.Header == "" {
		return fmt.Errorf("%w: the security scheme of the routes that need authentication needs a name and a "+
			"header", errInvalidSpec)
	}
	b.doc.Components.SecuritySchemes = map[string]securitySchemeObject{
		scheme.Name: {Type: "apiKey", Description: scheme.Description, Name: scheme.Header, In: "header"},
	}
	b.doc.Security = []securityRequirement{{scheme.Name: {}}}

	return nil
}

func (b *builder) addOperation(m mount, op Operation) error {
	method := strings.ToUpper(op.Method)
	if err := b.check(op, method); err != nil {
		return err
	}

	fullPath := documentPath(m.path, op.Pattern)
	item := b.doc.Paths[fullPath]
	if item == nil {
		item = map[string]*operationObject{}
		b.doc.Paths[fullPath] = item
	}
	key := strings.ToLower(method)
	if item[key] != nil {
		return errors.New("described twice")
	}

	obj, err := b.operationObject(m, op, method, fullPath)
	if err != nil {
		return err
	}
	item[key] = obj
	b.routes = append(b.routes, Route{Method: method, Path: fullPath})

	return nil
}

// check fails for an operation with an unknown method, without an ID or with one that another has, or
// without a summary or responses. It records the operation's ID as taken.
func (b *builder) check(op Operation, method string) error {
	if !slices.Contains(methods, method) {
		return fmt.Errorf("unknown method %q", op.Method)
	}
	if op.ID == "" || b.operationIDs[op.ID] {
		return fmt.Errorf("ID %q is empty or taken", op.ID)
	}
	b.operationIDs[op.ID] = true
	if op.Summary == "" {
		return errors.New("no summary")
	}
	if len(op.Responses) == 0 {
		return errors.New("no responses")
	}

	return nil
}

// operationObject returns the object of op, mounted with m at fullPath under method, with the parameters and
// problems that its body, its responses, m and the spec's cross-origin protection add.
func (b *builder) operationObject(m mount, op Operation, method, fullPath string) (*operationObject, error) {
	obj := &operationObject{
		Tags: []string{m.section.Name}, Summary: op.Summary, Description: op.Description, OperationID: op.ID,
		Responses: map[string]responseObject{},
	}
	if m.public {
		obj.Security = []securityRequirement{}
	}
	problems := slices.Clone(op.Problems)
	parameters := slices.Clone(op.Parameters)
	if op.Body != nil {
		body, err := b.requestBody(op.Body)
		if err != nil {
			return nil, err
		}
		obj.RequestBody = body
		problems = append(problems, envelope.BadRequestProblem, envelope.RequestEntityTooLargeProblem,
			envelope.UnsupportedMediaTypeProblem, request.InvalidBodyType)
	}
	for _, r := range op.Responses {
		if err := b.addResponse(obj, r); err != nil {
			return nil, err
		}
		if r.shape == collection {
			parameters = append(parameters, paginationParameters()...)
			problems = append(problems, request.InvalidQueryType)
		}
	}
	problems = append(problems, b.middlewareProblems(m, method)...)
	if err := addProblems(obj, problems); err != nil {
		return nil, err
	}
	params, err := parameterObjects(fullPath, parameters)
	if err != nil {
		return nil, err
	}
	obj.Parameters = params

	return obj, nil
}

// middlewareProblems returns the problems that the middleware in front of an operation mounted with m under
// method answers with: the 401 of authn.Require unless m is public, the 403 of
// middleware.CrossOriginProtection for an unsafe method if the spec is protected, and the 500.
func (b *builder) middlewareProblems(m mount, method string) []envelope.ProblemType {
	var problems []envelope.ProblemType
	if !m.public {
		problems = append(problems, envelope.UnauthorizedProblem)
	}
	if b.spec.CrossOriginProtected && slices.Contains(unsafeMethods, method) {
		problems = append(problems, middleware.CrossOriginRequestType)
	}

	return append(problems, envelope.InternalServerErrorProblem)
}

// documentPath returns the path of the document for the route pattern of an operation mounted at mountPath,
// without a trailing slash.
func documentPath(mountPath, pattern string) string {
	return path.Join("/", mountPath, pattern)
}

func (b *builder) requestBody(body *Body) (*requestBodyObject, error) {
	schema, err := b.ref(body.typ, typeschema.Input)
	if err != nil {
		return nil, fmt.Errorf("request body: %w", err)
	}

	return &requestBodyObject{
		Description: body.description, Required: true,
		Content: map[string]mediaTypeObject{body.mediaType: {Schema: schema}},
	}, nil
}

func (b *builder) addResponse(obj *operationObject, r Response) error {
	status := strconv.Itoa(r.status)
	if _, taken := obj.Responses[status]; taken {
		return fmt.Errorf("two responses with status %s", status)
	}

	res := responseObject{Description: r.description}
	if r.location {
		res.Headers = map[string]headerObject{"Location": {
			Description: "The path of the new resource.", Required: true,
			Schema: &jsonschema.Schema{Type: typeString, Format: formatURIReference},
		}}
	}
	if r.shape != noBody {
		schema, err := b.responseSchema(r)
		if err != nil {
			return fmt.Errorf("response %s: %w", status, err)
		}
		res.Content = map[string]mediaTypeObject{"application/json": {Schema: schema}}
	}
	obj.Responses[status] = res

	return nil
}

// responseSchema returns the schema of the body of r, a reference to the schema of its type in the
// envelope that its shape names.
func (b *builder) responseSchema(r Response) (*jsonschema.Schema, error) {
	data, err := b.ref(r.typ, typeschema.Output)
	if err != nil {
		return nil, err
	}

	switch r.shape {
	case resource:
		return object(map[string]*jsonschema.Schema{"data": data}), nil
	case collection:
		pagination := reflect.TypeFor[envelope.Pagination]()
		if err := b.addComponent(paginationSchemaName, pagination, typeschema.Output); err != nil {
			return nil, err
		}
		return object(map[string]*jsonschema.Schema{
			"data":       {Type: "array", Items: data},
			"pagination": refSchema(paginationSchemaName),
		}), nil
	case plain:
		return data, nil
	case noBody:
	}

	panic(fmt.Sprintf("openapi: response schema of shape %d", r.shape))
}

// object returns the schema of an object with the given properties, all required, and no others.
func object(properties map[string]*jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object", Properties: properties, Required: slices.Sorted(maps.Keys(properties)),
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
}

// ref adds the schema of t in the direction dir to the components, named like t, and returns a reference to
// it.
func (b *builder) ref(t reflect.Type, dir typeschema.Direction) (*jsonschema.Schema, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if err := b.addComponent(t.Name(), t, dir); err != nil {
		return nil, err
	}

	return refSchema(t.Name()), nil
}

// addComponent adds the schema of t in the direction dir to the components under name, unless it's there.
func (b *builder) addComponent(name string, t reflect.Type, dir typeschema.Direction) error {
	if !componentName.MatchString(name) {
		return fmt.Errorf("the name of %s isn't a valid component name", t)
	}
	if c, ok := b.components[name]; ok {
		if c.typ != t || c.dir != dir {
			return fmt.Errorf("two types or directions for the component %s: %s and %s", name, c.typ, t)
		}
		return nil
	}

	schema, err := typeschema.For(t, dir)
	if err != nil {
		return err //nolint:wrapcheck // Names t.
	}
	b.components[name] = component{typ: t, dir: dir}
	b.doc.Components.Schemas[name] = schema

	return nil
}

// refSchema returns a reference to the schema of the components named name.
func refSchema(name string) *jsonschema.Schema {
	return &jsonschema.Schema{Ref: componentsPath + name}
}

// parameterObjects returns the parameters of an operation with the path fullPath, failing unless they declare
// each parameter of the path's template, and no other path parameter.
func parameterObjects(fullPath string, parameters []Parameter) ([]parameterObject, error) {
	template, err := templateParameters(fullPath)
	if err != nil {
		return nil, err
	}

	objects := make([]parameterObject, 0, len(parameters))
	declared := map[string]bool{}
	for _, p := range parameters {
		if err := checkParameter(p, template, declared); err != nil {
			return nil, err
		}
		declared[p.In+" "+p.Name] = true
		objects = append(objects, parameterObject{
			Name: p.Name, In: p.In, Description: p.Description, Required: p.Required || p.In == inPath,
			Schema: p.Schema,
		})
	}
	for name := range template {
		if !declared[inPath+" "+name] {
			return nil, fmt.Errorf("path parameter %s isn't declared", name)
		}
	}

	return objects, nil
}

// checkParameter fails for a parameter p that's neither in the path nor in the query, has no schema, is in
// declared already, or is a path parameter that template, the parameters of the path, lacks.
func checkParameter(p Parameter, template, declared map[string]bool) error {
	switch {
	case p.In != inPath && p.In != inQuery:
		return fmt.Errorf("parameter %s is in %q, not in the path or query", p.Name, p.In)
	case p.Schema == nil:
		return fmt.Errorf("parameter %s has no schema", p.Name)
	case declared[p.In+" "+p.Name]:
		return fmt.Errorf("parameter %s is declared twice", p.Name)
	case p.In == inPath && !template[p.Name]:
		return fmt.Errorf("path parameter %s isn't in the path", p.Name)
	}

	return nil
}

// templateParameters returns the names of the parameters of the path template fullPath, failing for one with
// a regular expression.
func templateParameters(fullPath string) (map[string]bool, error) {
	names := map[string]bool{}
	for _, match := range templateParameter.FindAllStringSubmatch(fullPath, -1) {
		if strings.Contains(match[1], ":") {
			return nil, fmt.Errorf("path parameter %s has a regular expression", match[1])
		}
		names[match[1]] = true
	}

	return names, nil
}

// paginationParameters returns the query parameters of request.ParsePagination.
func paginationParameters() []Parameter {
	return []Parameter{
		{
			Name: "offset", In: inQuery, Description: "The zero-based position of the first resource of the page.",
			Schema: &jsonschema.Schema{Type: typeInteger, Minimum: new(0.0), Default: []byte(`0`)},
		},
		{
			Name: "limit", In: inQuery, Description: "The maximum number of resources in the page.",
			Schema: &jsonschema.Schema{
				Type: typeInteger, Minimum: new(1.0), Maximum: new(float64(request.MaxLimit)),
				Default: []byte(strconv.Itoa(request.DefaultLimit)),
			},
		},
	}
}

// addProblems adds the responses of the problem types to obj, one for each status, whose schema allows only
// the types of that status. It fails for a status that a successful response has.
func addProblems(obj *operationObject, problems []envelope.ProblemType) error {
	byStatus := map[int][]envelope.ProblemType{}
	for _, p := range problems {
		if !slices.Contains(byStatus[p.Status], p) {
			byStatus[p.Status] = append(byStatus[p.Status], p)
		}
	}

	for status, types := range byStatus {
		key := strconv.Itoa(status)
		if _, taken := obj.Responses[key]; taken {
			return fmt.Errorf("a problem has the status %s of a successful response", key)
		}
		slices.SortFunc(types, func(a, b envelope.ProblemType) int { return cmp.Compare(a.Type, b.Type) })

		uris := make([]any, len(types))
		lines := make([]string, len(types))
		for i, t := range types {
			uris[i] = t.Type
			lines[i] = fmt.Sprintf("- %s: %#q", t.Title, t.Type)
		}
		schema := &jsonschema.Schema{
			AllOf: []*jsonschema.Schema{refSchema(problemSchemaName)},
			Properties: map[string]*jsonschema.Schema{
				memberType:   {Enum: uris},
				memberStatus: {Const: new(any(status))},
			},
		}
		obj.Responses[key] = responseObject{
			Description: "A problem of one of these types:\n\n" + strings.Join(lines, "\n"),
			Content:     map[string]mediaTypeObject{envelope.ProblemJSONMediaType: {Schema: schema}},
		}
	}

	return nil
}

// problemSchema returns the schema of an envelope.Problem, which encodes itself.
func problemSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Description: "An RFC 9457 Problem Details object. Its type tells problems apart, and other members than " +
			"those below may extend it.",
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			memberType: {
				Type: typeString, Format: formatURIReference,
				Description: "Identifies the problem type, about:blank for a problem that means nothing beyond its " +
					"status.",
			},
			"title":      {Type: typeString, Description: "A summary of the problem type."},
			memberStatus: {Type: typeInteger, Description: "The HTTP status of the response."},
			"detail":     {Type: typeString, Description: "Explains this occurrence of the problem."},
			"instance":   {Type: typeString, Format: formatURIReference, Description: "Identifies this occurrence."},
			envelope.ValidationErrorsExtension: {
				Type:        "array",
				Description: "The invalid values of the request, for the problems about them.",
				Items:       refSchema(validationErrorSchemaName),
			},
		},
		Required: []string{memberType, "title", memberStatus},
	}
}
