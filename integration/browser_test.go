package integration_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/oauth2-proxy/mockoidc"
)

// chromeAvailable checks if Chrome/Chromium is installed
func chromeAvailable() bool {
	browsers := []string{
		"chromium-browser",
		"chromium",
		"google-chrome",
		"google-chrome-stable",
	}
	for _, browser := range browsers {
		if _, err := exec.LookPath(browser); err == nil {
			return true
		}
	}
	return false
}

// newBrowserContext creates a headless Chrome context
func newBrowserContext(t *testing.T) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true), // needed in containers/CI
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)

	// Combined cancel function
	cancel := func() {
		ctxCancel()
		allocCancel()
	}

	return ctx, cancel
}

// ============================================================================
// BROWSER TESTS
// ============================================================================

func TestBrowserPageLoads(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if !chromeAvailable() {
		t.Skip("skipping browser test: Chrome not available")
	}

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	// Set timeout for the whole test
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeoutCancel()

	var title string
	err := chromedp.Run(ctx,
		chromedp.Navigate(serverBaseURL),
		chromedp.Title(&title),
	)
	if err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

	t.Logf("Page title: %s", title)
}

func TestBrowserLoginPageElements(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if !chromeAvailable() {
		t.Skip("skipping browser test: Chrome not available")
	}

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeoutCancel()

	var pageContent string
	err := chromedp.Run(ctx,
		chromedp.Navigate(serverBaseURL),
		chromedp.OuterHTML("html", &pageContent),
	)
	if err != nil {
		t.Fatalf("Failed to get page content: %v", err)
	}

	// Verify page has content (even if it's our stub)
	if len(pageContent) < 50 {
		t.Errorf("Page content too short, got: %s", pageContent)
	}

	t.Logf("Page loaded, content length: %d bytes", len(pageContent))
}

func TestBrowserLoginRedirectCompletesFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if !chromeAvailable() {
		t.Skip("skipping browser test: Chrome not available")
	}

	// Queue a user for mock OIDC (browser will complete full flow)
	testUser := &mockoidc.MockUser{
		Subject:           "redirect-test-user",
		Email:             "redirect@example.com",
		PreferredUsername: "redirectuser",
		EmailVerified:     true,
	}
	mockOIDC.QueueUser(testUser)

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeoutCancel()

	var finalURL string
	err := chromedp.Run(ctx,
		// Navigate to login - browser will follow full redirect chain
		chromedp.Navigate(serverBaseURL+"/api/auth/login"),
		chromedp.Sleep(2*time.Second),
		chromedp.Location(&finalURL),
	)
	if err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// After full flow, should be back at the app
	if !strings.Contains(finalURL, "localhost:18080") {
		t.Errorf("Expected to end up at app after login flow, got: %s", finalURL)
	}

	t.Logf("Login flow completed, final URL: %s", finalURL)
}

func TestBrowserFullLoginFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if !chromeAvailable() {
		t.Skip("skipping browser test: Chrome not available")
	}

	// Queue a user for mock OIDC
	testUser := &mockoidc.MockUser{
		Subject:           "browser-test-user",
		Email:             "browser@example.com",
		PreferredUsername: "browseruser",
		EmailVerified:     true,
	}
	mockOIDC.QueueUser(testUser)

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	ctx, timeoutCancel := context.WithTimeout(ctx, 60*time.Second)
	defer timeoutCancel()

	var finalURL string
	var cookies []*network.Cookie

	err := chromedp.Run(ctx,
		// Step 1: Go to login
		chromedp.Navigate(serverBaseURL+"/api/auth/login"),
		chromedp.Sleep(1*time.Second),

		// Step 2: mockoidc auto-authenticates, follow redirects
		chromedp.WaitReady("body"),
		chromedp.Sleep(1*time.Second),

		// Get final location and cookies
		chromedp.Location(&finalURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	)
	if err != nil {
		t.Fatalf("Browser login flow failed: %v", err)
	}

	t.Logf("Final URL: %s", finalURL)
	t.Logf("Cookies: %d", len(cookies))

	// Check we got a session cookie
	hasSessionCookie := false
	for _, c := range cookies {
		t.Logf("  Cookie: %s", c.Name)
		if strings.Contains(c.Name, "session") {
			hasSessionCookie = true
		}
	}

	if !hasSessionCookie {
		t.Error("Expected session cookie after login")
	}
}
