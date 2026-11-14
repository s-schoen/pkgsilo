package app_test

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/apptest"
)

// compatResult is what the script of testdata/mcpcompat prints.
type compatResult struct {
	Initialize   jsontext.Value `json:"initialize"`
	Tools        []compatTool   `json:"tools"`
	Whoami       compatCall     `json:"whoami"`
	ToolError    compatCall     `json:"toolError"`
	Hidden       compatCall     `json:"hidden"`
	Unauthorized jsontext.Value `json:"unauthorized"`
}

// compatTool is a tool that the script lists.
type compatTool struct {
	Name        string         `json:"name"`
	Annotations jsontext.Value `json:"annotations"`
}

// compatCall is a tool call of the script.
type compatCall struct {
	IsError           bool           `json:"isError"`
	Text              string         `json:"text"`
	StructuredContent jsontext.Value `json:"structuredContent"`
}

// TestMCP_Compatibility checks the MCP endpoint against the official TypeScript SDK's client, which runs in
// a container with Node, built from testdata/mcpcompat. It reaches the app on the host through
// testcontainers' host port access, so that its requests reach a loopback listener with a Host header that
// isn't loopback, as behind a reverse proxy, which the SDK's localhost protection would refuse.
//
//  1. Create hidden, whose policies hide its own username, and denied, whose policies deny it reading
//     itself, each with an API key.
//  2. Run the script, which connects as admin and initializes: the server pkgsilo, with the tools
//     capability, at the newest protocol version that the TypeScript SDK speaks.
//  3. It lists the tools: identity_whoami among them, annotated as a read. The others are the features'
//     and left to TestMCP, so that this suite stays fixed while features add tools.
//  4. It calls identity_whoami as admin: admin, as the API lists it, in the structured output and the text.
//  5. It calls identity_whoami as denied: the tool error identity_not_found.
//  6. It calls identity_whoami as hidden: hidden without its username.
//  7. It connects without a key: refused with 401.
func TestMCP_Compatibility(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create the identities.
	hidden, asHidden := apptest.NewIdentity(api, "hidden")
	addPolicy(t, api, `forbid (principal == Identity::"`+hidden.ID.String()+`",
		action == Action::"identity:read:username", resource);`)
	denied, asDenied := apptest.NewIdentity(api, "denied")
	addPolicy(t, api, `forbid (principal == Identity::"`+denied.ID.String()+`",
		action == Action::"identity:read", resource);`)
	admin := onlyIdentity(t,
		api.Get("/api/v1/identities?limit=1").DecodeCollection[identityapi.IdentityResponse](http.StatusOK))

	// 2. to 7. Run the script.
	got := runCompatScript(t, api, map[string]string{
		"ADMIN_KEY":  api.Header().Get("X-Api-Key"),
		"HIDDEN_KEY": asHidden.Header().Get("X-Api-Key"),
		"DENIED_KEY": asDenied.Header().Get("X-Api-Key"),
	})

	// 2. Initialize.
	assert.JSONEq(t, `{"serverName":"pkgsilo","protocolVersion":"2025-11-25","capabilities":{"tools":{}}}`,
		string(got.Initialize))

	// 3. List the tools.
	whoami := slices.IndexFunc(got.Tools, func(tool compatTool) bool { return tool.Name == "identity_whoami" })
	if assert.GreaterOrEqual(t, whoami, 0, "identity_whoami isn't listed") {
		assert.JSONEq(t, `{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":true,"openWorldHint":false}`,
			string(got.Tools[whoami].Annotations))
	}

	// 4. Who am I, as admin.
	assert.False(t, got.Whoami.IsError, got.Whoami.Text)
	assert.JSONEq(t, marshalJSON(t, admin), string(got.Whoami.StructuredContent))
	assert.JSONEq(t, marshalJSON(t, admin), got.Whoami.Text)

	// 5. Who am I, as denied.
	assert.Equal(t, compatCall{
		IsError:           true,
		Text:              "identity_not_found: The identity does not exist, or the policies do not allow you to read it.",
		StructuredContent: jsontext.Value("null"),
	}, got.ToolError)

	// 6. Who am I, as hidden.
	assert.False(t, got.Hidden.IsError, got.Hidden.Text)
	var hiddenOut map[string]any
	require.NoError(t, json.Unmarshal(got.Hidden.StructuredContent, &hiddenOut))
	assert.Equal(t, hidden.ID.String(), hiddenOut["id"])
	assert.Equal(t, "hidden", hiddenOut["displayName"])
	assert.NotContains(t, hiddenOut, "username")

	// 7. Without a key.
	assert.JSONEq(t, `{"name":"StreamableHTTPError","code":401}`, string(got.Unauthorized))
}

// runCompatScript serves the app of api on a loopback listener and runs the script of testdata/mcpcompat
// against it with the environment env and MCP_URL, and returns what it printed. It stops the test if the
// script fails.
func runCompatScript(t *testing.T, api *apitest.Client, env map[string]string) compatResult {
	t.Helper()

	server := httptest.NewServer(api.Handler())
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(serverURL.Port())
	require.NoError(t, err)
	env["MCP_URL"] = "http://" + testcontainers.HostInternal + ":" + serverURL.Port() + "/mcp"

	ctx := t.Context()
	ctr, err := testcontainers.Run(ctx, "",
		testcontainers.WithDockerfile(testcontainers.FromDockerfile{Context: "testdata/mcpcompat", KeepImage: true}),
		testcontainers.WithEnv(env),
		testcontainers.WithHostPortAccess(port),
		// The tunnel to the host opens once the container is ready, and the script waits for it.
		testcontainers.WithWaitStrategy(wait.ForLog("waiting for pkgsilo")),
	)
	testcontainers.CleanupContainer(t, ctr)
	require.NoError(t, err, "run the compatibility script")
	require.NoError(t, wait.ForExit().WaitUntilReady(ctx, ctr), "wait for the compatibility script")

	logs, err := ctr.Logs(ctx)
	require.NoError(t, err)
	output, err := io.ReadAll(logs)
	require.NoError(t, err)
	state, err := ctr.State(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, state.ExitCode, "exit code of the script, whose output is:\n%s", output)

	// The script prints its result last, after what Node and the script write to stderr.
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result compatResult
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &result), "output:\n%s", output)

	return result
}

// addPolicy saves a policy with a random name through the client api, which may write policies.
func addPolicy(t *testing.T, api *apitest.Client, source string) {
	t.Helper()

	body, err := json.Marshal(map[string]string{"name": uuid.NewV4().String(), "source": source})
	require.NoError(t, err)
	api.Post("/api/v1/policies", string(body)).AssertStatus(http.StatusCreated)
}

// marshalJSON returns v as JSON.
func marshalJSON(t *testing.T, v any) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, json.MarshalWrite(&buf, v))

	return buf.String()
}
