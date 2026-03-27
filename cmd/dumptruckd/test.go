package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Saadlulu/dumptruckd/pkg/config"
	"github.com/Saadlulu/dumptruckd/pkg/test"
)

func runTest(configPath string, verbose bool) {
	fmt.Println("🧪 dumptruckd Configuration Tester")
	fmt.Println("===================================")
	fmt.Println()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create tester
	tester := test.NewTester(cfg)

	// Run tests with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Println("Running tests...")
	fmt.Println()
	results, err := tester.TestAll(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Test execution failed: %v\n", err)
		os.Exit(1)
	}

	// Print results
	passed := 0
	failed := 0
	skipped := 0

	fmt.Println("Test Results:")
	fmt.Println("-------------")

	for _, result := range results {
		switch result.Status {
		case "pass":
			fmt.Printf("✅ %s: %s\n", result.Component, result.Message)
			passed++
		case "fail":
			fmt.Printf("❌ %s: %s\n", result.Component, result.Message)
			if verbose && result.Error != nil {
				fmt.Printf("   Error: %v\n", result.Error)
			}
			failed++
		case "skip":
			fmt.Printf("⏭️  %s: %s\n", result.Component, result.Message)
			skipped++
		default:
			fmt.Printf("❓ %s: %s\n", result.Component, result.Message)
		}
	}

	fmt.Println("\n-------------")
	fmt.Printf("Summary: %d passed, %d failed, %d skipped\n", passed, failed, skipped)

	if failed > 0 {
		fmt.Println("\n⚠️  Some tests failed. Please check your configuration.")
		os.Exit(1)
	}

	fmt.Println("\n✅ All tests passed! Your configuration is ready to use.")
}
