package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start development servers",
	Long: `Starts both the Go backend and Vite frontend development servers.

The command will:
  1. Start Vite dev server (npm run dev) in frontend/
  2. Start Go server (go run) with the main package
  3. Handle Ctrl+C for graceful shutdown of both`,
	RunE: runDev,
}

func runDev(cmd *cobra.Command, args []string) error {
	// Check if we're in a juango project
	if !isJuangoProject() {
		return fmt.Errorf("not a juango project (missing go.mod or frontend/package.json)")
	}

	// Find the project name from go.mod
	projectName, err := getProjectName()
	if err != nil {
		return fmt.Errorf("getting project name: %w", err)
	}

	fmt.Printf("Starting development servers for %s...\n\n", projectName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Start Vite dev server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := startVite(ctx); err != nil && ctx.Err() == nil {
			errChan <- fmt.Errorf("vite: %w", err)
		}
	}()

	// Start Go server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := startGo(ctx, projectName); err != nil && ctx.Err() == nil {
			errChan <- fmt.Errorf("go: %w", err)
		}
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived %s, shutting down...\n", sig)
		cancel()
	case err := <-errChan:
		fmt.Printf("\nError: %v\n", err)
		cancel()
	}

	// Wait for processes to finish
	wg.Wait()
	fmt.Println("Development servers stopped")
	return nil
}

func isJuangoProject() bool {
	// Check for go.mod
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		return false
	}

	// Check for frontend/package.json
	if _, err := os.Stat("frontend/package.json"); os.IsNotExist(err) {
		return false
	}

	return true
}

func getProjectName() (string, error) {
	// Get from directory name
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Base(cwd), nil
}

func startVite(ctx context.Context) error {
	fmt.Println("Starting Vite dev server...")

	cmd := exec.CommandContext(ctx, "npm", "run", "dev")
	cmd.Dir = "frontend"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set platform-specific process attributes
	setProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	// Wait for context cancellation or process exit
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		killProcess(cmd)
		return nil
	case err := <-done:
		return err
	}
}

func startGo(ctx context.Context, projectName string) error {
	fmt.Println("Starting Go server...")

	// Find the main.go file
	mainFile := fmt.Sprintf("cmd/%s/%s.go", projectName, projectName)
	if _, err := os.Stat(mainFile); os.IsNotExist(err) {
		// Try alternative location
		mainFile = "main.go"
		if _, err := os.Stat(mainFile); os.IsNotExist(err) {
			return fmt.Errorf("cannot find main.go (tried cmd/%s/%s.go and main.go)", projectName, projectName)
		}
	}

	cmd := exec.CommandContext(ctx, "go", "run", mainFile, "serve")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set platform-specific process attributes
	setProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	// Wait for context cancellation or process exit
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		killProcess(cmd)
		return nil
	case err := <-done:
		return err
	}
}
