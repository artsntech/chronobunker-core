package graph_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/agent"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/graph"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/middleware"
)

const (
	tenantAcme   = "11111111-1111-1111-1111-111111111111"
	tenantGlobex = "22222222-2222-2222-2222-222222222222"
	agentAcmeID  = "5ca2ccb5-061f-4042-9222-9c076776a2e3"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("CHRONO_TEST_DB_URL")
	if url == "" {
		t.Skip("CHRONO_TEST_DB_URL not set")
	}
	conn, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func issueToken(t *testing.T, key *rsa.PrivateKey, tenantID string, expiry time.Duration) string {
	t.Helper()
	claims := &middleware.Claims{
		TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "chronobunker",
			Audience:  jwt.ClaimStrings{"chronobunker-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// graphqlRequest sends a POST to the GraphQL endpoint. The variables map may
// be nil for queries without variables.
func graphqlRequest(t *testing.T, srv http.Handler, token, query string, variables map[string]any) map[string]any {
	t.Helper()
	body := map[string]any{"query": query}
	if variables != nil {
		body["variables"] = variables
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
	}
	return resp
}

func TestGraphQLTenantIsolationEndToEnd(t *testing.T) {
	database := testDB(t)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	auth, err := middleware.NewAuthFromKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("init auth: %v", err)
	}

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{AgentRepo: agent.NewRepository(database), DB: database},
	}))

	handler := auth.Middleware(srv)

	acmeToken := issueToken(t, privateKey, tenantAcme, time.Hour)
	resp := graphqlRequest(t, handler, acmeToken, `{ agents { name tenantId } }`, nil)

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected response shape: %v", resp)
	}
	agents, ok := data["agents"].([]any)
	if !ok || len(agents) == 0 {
		t.Fatal("expected acme to have at least one agent")
	}
	for _, raw := range agents {
		a := raw.(map[string]any)
		if a["tenantId"] != tenantAcme {
			t.Errorf("SECURITY: acme saw an agent from tenant %s", a["tenantId"])
		}
	}

	globexToken := issueToken(t, privateKey, tenantGlobex, time.Hour)
	resp = graphqlRequest(t, handler, globexToken, `{ agents { name tenantId } }`, nil)
	data = resp["data"].(map[string]any)
	agents = data["agents"].([]any)
	for _, raw := range agents {
		a := raw.(map[string]any)
		if a["tenantId"] != tenantGlobex {
			t.Errorf("SECURITY: globex saw an agent from tenant %s", a["tenantId"])
		}
	}

	expiredToken := issueToken(t, privateKey, tenantAcme, -time.Hour)
	resp = graphqlRequest(t, handler, expiredToken, `{ agents { name } }`, nil)
	if _, hasData := resp["data"]; hasData {
		t.Fatal("SECURITY: expired token was accepted")
	}

	wrongKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	wrongToken := issueToken(t, wrongKey, tenantAcme, time.Hour)
	resp = graphqlRequest(t, handler, wrongToken, `{ agents { name } }`, nil)
	if _, hasData := resp["data"]; hasData {
		t.Fatal("SECURITY: token signed with wrong key was accepted")
	}
}