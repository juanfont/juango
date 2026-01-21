package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/oauth2-proxy/mockoidc"
)

// Shared test infrastructure
var (
	testServer    *testServerInfo
	mockOIDC      *mockoidc.MockOIDC
	serverBaseURL string
)

type testServerInfo struct {
	tmpDir     string
	projectDir string
	cmd        *exec.Cmd
	cancel     context.CancelFunc
	output     *bytes.Buffer
}

// projectRoot returns the root directory of the juango project
func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filename))
}

func TestMain(m *testing.M) {
	// Parse flags first (required before calling testing.Short())
	flag.Parse()

	// Skip setup in short mode
	if testing.Short() {
		os.Exit(m.Run())
	}

	var exitCode int
	defer func() {
		teardown()
		os.Exit(exitCode)
	}()

	if err := setup(); err != nil {
		fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
		exitCode = 1
		return
	}

	exitCode = m.Run()
}

func setup() error {
	fmt.Println("=== SETUP: Building test infrastructure ===")

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "juango-integration-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	projectName := "testapp"
	projectDir := filepath.Join(tmpDir, projectName)

	// Build the juango CLI
	fmt.Println("Building juango CLI...")
	juangoBin := filepath.Join(tmpDir, "juango")
	buildCmd := exec.Command("go", "build", "-o", juangoBin, ".")
	buildCmd.Dir = projectRoot()
	if output, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to build juango CLI: %w\n%s", err, output)
	}

	// Initialize the project
	fmt.Println("Initializing test project...")
	initCmd := exec.Command(juangoBin, "init", projectName, "-m", "github.com/test/"+projectName)
	initCmd.Dir = tmpDir
	if output, err := initCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to init project: %w\n%s", err, output)
	}

	// Start mock OIDC provider
	fmt.Println("Starting mock OIDC provider...")
	mockOIDC, err = mockoidc.Run()
	if err != nil {
		return fmt.Errorf("failed to start mock OIDC: %w", err)
	}

	// Create config file
	configPath := filepath.Join(projectDir, "config.yml")
	config := fmt.Sprintf(`
listen_addr: ":18080"
advertise_url: "http://localhost:18080"
admin_mode_timeout: 30m

database:
  path: "%s/test.db"

session:
  cookie_name: "testapp_session"
  cookie_expiry: 24h
  authentication_key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  encryption_key: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

oidc:
  issuer: "%s"
  client_id: "%s"
  client_secret: "%s"
  scopes:
    - openid
    - profile
    - email

redis:
  addr: "localhost:6379"

logging:
  level: debug
  format: text
`, projectDir, mockOIDC.Issuer(), mockOIDC.ClientID, mockOIDC.ClientSecret)

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Create minimal frontend dist directory
	fmt.Println("Creating frontend dist...")
	distDir := filepath.Join(projectDir, "frontend", "dist")
	if err := os.MkdirAll(distDir, 0755); err != nil {
		return fmt.Errorf("failed to create dist dir: %w", err)
	}

	indexHTML := `<!DOCTYPE html><html><head><title>Test</title></head><body><div id="root">Test App</div></body></html>`
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte(indexHTML), 0644); err != nil {
		return fmt.Errorf("failed to write index.html: %w", err)
	}

	// Add replace directive for local juango
	fmt.Println("Configuring go.mod...")
	goModPath := filepath.Join(projectDir, "go.mod")
	goModContent, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}
	replaceDirective := fmt.Sprintf("\nreplace github.com/juanfont/juango => %s\n", projectRoot())
	if err := os.WriteFile(goModPath, append(goModContent, []byte(replaceDirective)...), 0644); err != nil {
		return fmt.Errorf("failed to update go.mod: %w", err)
	}

	// Run go mod tidy
	fmt.Println("Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = projectDir
	if output, err := tidyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run go mod tidy: %w\n%s", err, output)
	}

	// Build the project
	fmt.Println("Building test project...")
	projectBin := filepath.Join(projectDir, projectName)
	goBuildCmd := exec.Command("go", "build", "-o", projectBin, "./cmd/"+projectName)
	goBuildCmd.Dir = projectDir
	if output, err := goBuildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to build project: %w\n%s", err, output)
	}

	// Start the server
	fmt.Println("Starting test server...")
	ctx, cancel := context.WithCancel(context.Background())

	serverCmd := exec.CommandContext(ctx, projectBin, "serve", "-c", configPath)
	serverCmd.Dir = projectDir

	var serverOutput bytes.Buffer
	serverCmd.Stdout = &serverOutput
	serverCmd.Stderr = &serverOutput

	if err := serverCmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start server: %w", err)
	}

	testServer = &testServerInfo{
		tmpDir:     tmpDir,
		projectDir: projectDir,
		cmd:        serverCmd,
		cancel:     cancel,
		output:     &serverOutput,
	}
	serverBaseURL = "http://localhost:18080"

	// Wait for server to be ready
	fmt.Println("Waiting for server to be ready...")
	if !waitForServer(serverBaseURL+"/api/auth/session", 10*time.Second) {
		return fmt.Errorf("server did not become ready in time.\nOutput: %s", serverOutput.String())
	}

	fmt.Println("=== SETUP COMPLETE ===")
	return nil
}

func teardown() {
	fmt.Println("\n=== TEARDOWN ===")

	if testServer != nil {
		fmt.Println("Stopping test server...")
		testServer.cancel()
		testServer.cmd.Wait()

		fmt.Println("Cleaning up temp directory...")
		os.RemoveAll(testServer.tmpDir)
	}

	if mockOIDC != nil {
		fmt.Println("Stopping mock OIDC...")
		mockOIDC.Shutdown()
	}

	fmt.Println("=== TEARDOWN COMPLETE ===")
}

func waitForServer(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// newClient creates a new HTTP client with cookie jar
func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// ============================================================================
// TESTS
// ============================================================================

func TestSessionUnauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := newClient()

	resp, err := client.Get(serverBaseURL + "/api/auth/session")
	if err != nil {
		t.Fatalf("Failed to check session: %v", err)
	}
	defer resp.Body.Close()

	var sessionResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		t.Fatalf("Failed to decode session response: %v", err)
	}

	if sessionResp["authenticated"] != false {
		t.Errorf("Expected unauthenticated, got: %v", sessionResp)
	}
}

func TestLoginRedirectsToOIDC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := newClient()

	resp, err := client.Get(serverBaseURL + "/api/auth/login")
	if err != nil {
		t.Fatalf("Failed to call login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusTemporaryRedirect {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected redirect from login, got %d: %s", resp.StatusCode, body)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, mockOIDC.Issuer()) {
		t.Errorf("Expected redirect to OIDC issuer, got: %s", location)
	}
}

func TestProtectedEndpointReturns401(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := newClient()

	resp, err := client.Get(serverBaseURL + "/api/admin/mode/status")
	if err != nil {
		t.Fatalf("Failed to call admin endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 401 from protected endpoint, got %d: %s", resp.StatusCode, body)
	}
}

func TestOIDCLoginFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Queue a user for the mock OIDC to return
	testUser := &mockoidc.MockUser{
		Subject:           "test-user-123",
		Email:             "test@example.com",
		PreferredUsername: "testuser",
		EmailVerified:     true,
	}
	mockOIDC.QueueUser(testUser)

	client := newClient()

	// Step 1: Call login endpoint and get OIDC redirect
	t.Log("Step 1: Starting OIDC login flow...")
	loginResp, err := client.Get(serverBaseURL + "/api/auth/login")
	if err != nil {
		t.Fatalf("Failed to call login: %v", err)
	}
	loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusFound && loginResp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("Expected redirect from login, got %d", loginResp.StatusCode)
	}

	oidcAuthURL := loginResp.Header.Get("Location")
	t.Logf("  Redirected to OIDC: %s...", oidcAuthURL[:min(80, len(oidcAuthURL))])

	// Step 2: Follow redirect to mock OIDC and authenticate
	t.Log("Step 2: Authenticating with mock OIDC...")
	authResp, err := client.Get(oidcAuthURL)
	if err != nil {
		t.Fatalf("Failed to call OIDC auth: %v", err)
	}
	authResp.Body.Close()

	if authResp.StatusCode != http.StatusFound && authResp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("Expected redirect from OIDC auth, got %d", authResp.StatusCode)
	}

	callbackURL := authResp.Header.Get("Location")
	t.Logf("  OIDC callback: %s...", callbackURL[:min(80, len(callbackURL))])

	// Step 3: Follow callback redirect
	t.Log("Step 3: Following callback redirect...")
	callbackResp, err := client.Get(callbackURL)
	if err != nil {
		t.Fatalf("Failed to call callback: %v", err)
	}
	callbackResp.Body.Close()

	t.Logf("  Callback response status: %d", callbackResp.StatusCode)

	// Step 4: Verify session is now authenticated
	t.Log("Step 4: Verifying session is authenticated...")
	sessionResp, err := client.Get(serverBaseURL + "/api/auth/session")
	if err != nil {
		t.Fatalf("Failed to check session: %v", err)
	}
	defer sessionResp.Body.Close()

	var sessionData map[string]interface{}
	if err := json.NewDecoder(sessionResp.Body).Decode(&sessionData); err != nil {
		t.Fatalf("Failed to decode session response: %v", err)
	}

	if sessionData["authenticated"] != true {
		t.Logf("Session data: %+v", sessionData)
		t.Fatalf("Expected authenticated=true after login, got: %v", sessionData["authenticated"])
	}

	t.Log("  Session is authenticated!")

	// Verify user data
	if user, ok := sessionData["user"].(map[string]interface{}); ok {
		if user["email"] != "test@example.com" {
			t.Errorf("Expected email test@example.com, got %v", user["email"])
		}
		t.Logf("  User email: %v", user["email"])
	} else {
		t.Error("User data not found in session")
	}
}

func TestLogout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Queue a user and login first
	testUser := &mockoidc.MockUser{
		Subject:           "logout-test-user",
		Email:             "logout@example.com",
		PreferredUsername: "logoutuser",
		EmailVerified:     true,
	}
	mockOIDC.QueueUser(testUser)

	client := newClient()

	// Login
	loginResp, _ := client.Get(serverBaseURL + "/api/auth/login")
	loginResp.Body.Close()
	authResp, _ := client.Get(loginResp.Header.Get("Location"))
	authResp.Body.Close()
	callbackResp, _ := client.Get(authResp.Header.Get("Location"))
	callbackResp.Body.Close()

	// Verify logged in
	sessionResp, _ := client.Get(serverBaseURL + "/api/auth/session")
	var sessionData map[string]interface{}
	json.NewDecoder(sessionResp.Body).Decode(&sessionData)
	sessionResp.Body.Close()

	if sessionData["authenticated"] != true {
		t.Fatal("Expected to be authenticated before logout")
	}

	// Logout
	t.Log("Logging out...")
	logoutResp, err := client.Post(serverBaseURL+"/api/auth/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to logout: %v", err)
	}
	logoutResp.Body.Close()

	// Verify logged out
	sessionResp2, _ := client.Get(serverBaseURL + "/api/auth/session")
	var sessionData2 map[string]interface{}
	json.NewDecoder(sessionResp2.Body).Decode(&sessionData2)
	sessionResp2.Body.Close()

	if sessionData2["authenticated"] != false {
		t.Errorf("Expected unauthenticated after logout, got: %v", sessionData2)
	}

	t.Log("Logout successful!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
