package openapi

import "github.com/google/jsonschema-go/jsonschema"

// The objects of an OpenAPI 3.1 document that Build writes, named as in the specification, with only the
// members that it uses.

type documentObject struct {
	OpenAPI    string                `json:"openapi"` //nolint:tagliatelle // Named by the OpenAPI specification.
	Info       infoObject            `json:"info"`
	Tags       []tagObject           `json:"tags,omitempty"`
	Paths      pathsObject           `json:"paths"`
	Components componentsObject      `json:"components"`
	Security   []securityRequirement `json:"security,omitempty"`
}

// pathsObject holds the operations by path and then by method, in lower case.
type pathsObject map[string]map[string]*operationObject

type infoObject struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

type tagObject struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type operationObject struct {
	Tags        []string                  `json:"tags,omitempty"`
	Summary     string                    `json:"summary,omitempty"`
	Description string                    `json:"description,omitempty"`
	OperationID string                    `json:"operationId"`
	Parameters  []parameterObject         `json:"parameters,omitempty"`
	RequestBody *requestBodyObject        `json:"requestBody,omitempty"`
	Responses   map[string]responseObject `json:"responses"`
	// Security is empty, but not nil, for a public operation, which overrides the document's.
	Security []securityRequirement `json:"security,omitzero"`
}

type parameterObject struct {
	Name        string             `json:"name"`
	In          string             `json:"in"`
	Description string             `json:"description,omitempty"`
	Required    bool               `json:"required,omitzero"`
	Schema      *jsonschema.Schema `json:"schema"`
}

type requestBodyObject struct {
	Description string                     `json:"description,omitempty"`
	Required    bool                       `json:"required"`
	Content     map[string]mediaTypeObject `json:"content"`
}

type responseObject struct {
	Description string                     `json:"description"`
	Headers     map[string]headerObject    `json:"headers,omitempty"`
	Content     map[string]mediaTypeObject `json:"content,omitempty"`
}

type headerObject struct {
	Description string             `json:"description,omitempty"`
	Required    bool               `json:"required,omitzero"`
	Schema      *jsonschema.Schema `json:"schema"`
}

type mediaTypeObject struct {
	Schema *jsonschema.Schema `json:"schema"`
}

type componentsObject struct {
	Schemas         map[string]*jsonschema.Schema   `json:"schemas,omitempty"`
	SecuritySchemes map[string]securitySchemeObject `json:"securitySchemes,omitempty"`
}

type securitySchemeObject struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Name        string `json:"name"`
	In          string `json:"in"`
}

// securityRequirement maps the name of a security scheme to its scopes, which an API key has none of.
type securityRequirement map[string][]string
