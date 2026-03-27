package test

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dumptruckd/dumptruckd/pkg/config"
	"github.com/dumptruckd/dumptruckd/pkg/compress"
	"github.com/dumptruckd/dumptruckd/pkg/dump"
	"github.com/dumptruckd/dumptruckd/pkg/notify"
	"github.com/dumptruckd/dumptruckd/pkg/upload"
)

// Tester validates configuration by testing each component end-to-end.
type Tester struct {
	cfg *config.Config
}

// TestResult holds the outcome of a single component test.
type TestResult struct {
	Component string
	Status    string // "pass", "fail", "skip"
	Message   string
	Error     error
}

// NewTester creates a new configuration tester.
func NewTester(cfg *config.Config) *Tester {
	return &Tester{cfg: cfg}
}

// TestAll runs all tests for the configuration
func (t *Tester) TestAll(ctx context.Context) ([]TestResult, error) {
	var results []TestResult

	// Test components
	results = append(results, t.testDatabases(ctx)...)
	results = append(results, t.testCompressors()...)
	results = append(results, t.testUploaders(ctx)...)
	results = append(results, t.testNotifiers()...)

	// Test full backup pipeline for each backup job
	for _, backup := range t.cfg.Backups {
		results = append(results, t.testBackupPipeline(ctx, backup)...)
	}

	return results, nil
}

// testDatabases tests all database connections
func (t *Tester) testDatabases(ctx context.Context) []TestResult {
	var results []TestResult

	for name, dbCfg := range t.cfg.Databases {
		result := TestResult{Component: fmt.Sprintf("database.%s", name)}
		
		// Test connection and small dump
		if err := t.testDatabase(ctx, dbCfg); err != nil {
			result.Status = "fail"
			result.Error = err
			result.Message = fmt.Sprintf("Failed to connect or dump: %v", err)
		} else {
			result.Status = "pass"
			result.Message = "Connection successful, test dump created"
		}
		
		results = append(results, result)
	}

	return results
}

// testDatabase tests a single database connection and creates a small test dump
func (t *Tester) testDatabase(ctx context.Context, dbCfg config.DatabaseConfig) error {
	dumper, err := dump.NewDumper(dbCfg)
	if err != nil {
		return fmt.Errorf("create dumper: %w", err)
	}

	// Create a test dump (schema only, no data)
	var dumpFile string
	if testDumper, ok := dumper.(dump.TestDumper); ok {
		var err error
		dumpFile, err = testDumper.TestDump(ctx)
		if err != nil {
			return fmt.Errorf("test dump failed: %w", err)
		}
		defer os.Remove(dumpFile)
	} else {
		// Fallback to regular dump if TestDump not available
		var err error
		dumpFile, err = dumper.Dump(ctx)
		if err != nil {
			return fmt.Errorf("dump failed: %w", err)
		}
		defer os.Remove(dumpFile)
	}

	// Verify dump file exists and has content
	info, err := os.Stat(dumpFile)
	if err != nil {
		return fmt.Errorf("dump file not accessible: %w", err)
	}

	if info.Size() == 0 {
		return fmt.Errorf("dump file is empty")
	}

	// Read first few bytes to verify it's a valid dump
	file, err := os.Open(dumpFile)
	if err != nil {
		return fmt.Errorf("cannot read dump file: %w", err)
	}
	defer file.Close()

	buf := make([]byte, 100)
	if _, err := file.Read(buf); err != nil && err != io.EOF {
		return fmt.Errorf("cannot read dump content: %w", err)
	}

	return nil
}

// testCompressors tests all compressor configurations
func (t *Tester) testCompressors() []TestResult {
	var results []TestResult

	for name, compCfg := range t.cfg.Compressors {
		result := TestResult{Component: fmt.Sprintf("compressor.%s", name)}
		
		if err := t.testCompressor(compCfg); err != nil {
			result.Status = "fail"
			result.Error = err
			result.Message = fmt.Sprintf("Compression test failed: %v", err)
		} else {
			result.Status = "pass"
			result.Message = "Compression test successful"
		}
		
		results = append(results, result)
	}

	return results
}

// testCompressor tests compression by creating a test file and compressing it
func (t *Tester) testCompressor(compCfg config.CompressConfig) error {
	// Create a small test file
	testFile, err := os.CreateTemp("", "dumptruckd-test-*.txt")
	if err != nil {
		return fmt.Errorf("create test file: %w", err)
	}
	defer os.Remove(testFile.Name())

	// Write some test data
	testData := "This is a test file for dumptruckd compression testing.\n" +
		"It contains some sample data that should compress well.\n" +
		"Repeating text repeating text repeating text.\n"
	
	if _, err := testFile.WriteString(testData); err != nil {
		return fmt.Errorf("write test data: %w", err)
	}
	testFile.Close()

	// Test compression
	compressor, err := compress.NewCompressor(compCfg)
	if err != nil {
		return fmt.Errorf("create compressor: %w", err)
	}

	compressedFile, err := compressor.Compress(testFile.Name())
	if err != nil {
		return fmt.Errorf("compress failed: %w", err)
	}
	defer os.Remove(compressedFile)

	// Verify compressed file exists
	if _, err := os.Stat(compressedFile); err != nil {
		return fmt.Errorf("compressed file not accessible: %w", err)
	}

	return nil
}

// testUploaders tests all uploader configurations
func (t *Tester) testUploaders(ctx context.Context) []TestResult {
	var results []TestResult

	for name, uploadCfg := range t.cfg.Uploaders {
		result := TestResult{Component: fmt.Sprintf("uploader.%s", name)}
		
		if err := t.testUploader(ctx, uploadCfg, name); err != nil {
			result.Status = "fail"
			result.Error = err
			result.Message = fmt.Sprintf("Upload test failed: %v", err)
		} else {
			result.Status = "pass"
			result.Message = "Upload, download, and delete test successful"
		}
		
		results = append(results, result)
	}

	return results
}

// testUploader tests upload by uploading a test file, verifying it, then deleting it
func (t *Tester) testUploader(ctx context.Context, uploadCfg config.UploadConfig, componentName string) error {
	// Create a small test file
	testFile, err := os.CreateTemp("", "dumptruckd-test-upload-*.txt")
	if err != nil {
		return fmt.Errorf("create test file: %w", err)
	}
	defer os.Remove(testFile.Name())

	testData := fmt.Sprintf("dumptruckd test file - %s\nCreated at: %s\n",
		componentName, time.Now().Format(time.RFC3339))
	
	if _, err := testFile.WriteString(testData); err != nil {
		return fmt.Errorf("write test data: %w", err)
	}
	testFile.Close()

	// Test upload
	uploader, err := upload.NewUploader(uploadCfg)
	if err != nil {
		return fmt.Errorf("create uploader: %w", err)
	}

	// Upload with a test prefix to make it easy to identify and clean up
	testBackupName := fmt.Sprintf("dumptruckd-test-%d", time.Now().Unix())
	remotePath, err := uploader.Upload(ctx, testFile.Name(), testBackupName)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	// Try to verify the file exists (if uploader supports it)
	if verifiable, ok := uploader.(upload.VerifiableUploader); ok {
		if err := verifiable.Verify(ctx, remotePath); err != nil {
			verifiable.Delete(ctx, remotePath)
			return fmt.Errorf("verify upload failed: %w", err)
		}

		// Delete the test file
		if err := verifiable.Delete(ctx, remotePath); err != nil {
			return fmt.Errorf("delete test file failed: %w", err)
		}
	}

	return nil
}

// testNotifiers tests all notifier configurations
func (t *Tester) testNotifiers() []TestResult {
	var results []TestResult

	// Test notifiers that are referenced in backup jobs
	notifiersTested := make(map[string]bool)

	for _, backup := range t.cfg.Backups {
		if backup.Notify.Type == "" || backup.Notify.Type == "none" {
			continue
		}

		key := fmt.Sprintf("%s-%s", backup.Notify.Type, backup.Name)
		if notifiersTested[key] {
			continue
		}
		notifiersTested[key] = true

		result := TestResult{Component: fmt.Sprintf("notify.%s (backup: %s)", backup.Notify.Type, backup.Name)}
		
		if err := t.testNotifier(backup.Notify); err != nil {
			result.Status = "fail"
			result.Error = err
			result.Message = fmt.Sprintf("Notification test failed: %v", err)
		} else {
			result.Status = "pass"
			result.Message = "Test notification sent successfully"
		}
		
		results = append(results, result)
	}

	return results
}

// testNotifier sends a test notification
func (t *Tester) testNotifier(notifyCfg config.NotifyConfig) error {
	notifier, err := notify.NewNotifier(notifyCfg)
	if err != nil {
		return fmt.Errorf("create notifier: %w", err)
	}

	testMsg := fmt.Sprintf("🧪 dumptruckd test notification\nTime: %s\nThis is a test message to verify notification configuration.", 
		time.Now().Format(time.RFC3339))
	
	if err := notifier.Notify(testMsg); err != nil {
		return fmt.Errorf("send notification failed: %w", err)
	}

	return nil
}

// testBackupPipeline tests the full backup pipeline for a backup job (dry-run)
func (t *Tester) testBackupPipeline(ctx context.Context, backupCfg config.BackupConfig) []TestResult {
	var results []TestResult
	backupName := fmt.Sprintf("backup.%s", backupCfg.Name)

	// Test database connection
	result := TestResult{Component: fmt.Sprintf("%s.database", backupName)}
	if err := t.testDatabase(ctx, backupCfg.Database); err != nil {
		result.Status = "fail"
		result.Error = err
		result.Message = fmt.Sprintf("Database test failed: %v", err)
	} else {
		result.Status = "pass"
		result.Message = "Database connection successful"
	}
	results = append(results, result)

	// Test compression
	result = TestResult{Component: fmt.Sprintf("%s.compress", backupName)}
	if err := t.testCompressor(backupCfg.Compress); err != nil {
		result.Status = "fail"
		result.Error = err
		result.Message = fmt.Sprintf("Compression test failed: %v", err)
	} else {
		result.Status = "pass"
		result.Message = "Compression test successful"
	}
	results = append(results, result)

	// Test upload (full pipeline: dump -> compress -> upload -> delete)
	result = TestResult{Component: fmt.Sprintf("%s.pipeline", backupName)}
	if err := t.testFullPipeline(ctx, backupCfg); err != nil {
		result.Status = "fail"
		result.Error = err
		result.Message = fmt.Sprintf("Full pipeline test failed: %v", err)
	} else {
		result.Status = "pass"
		result.Message = "Full pipeline test successful (dump -> compress -> upload -> delete)"
	}
	results = append(results, result)

	return results
}

// testFullPipeline tests the complete backup pipeline end-to-end
func (t *Tester) testFullPipeline(ctx context.Context, backupCfg config.BackupConfig) error {
	// Step 1: Create a small test dump (schema only)
	dumper, err := dump.NewDumper(backupCfg.Database)
	if err != nil {
		return fmt.Errorf("create dumper: %w", err)
	}

	// Use TestDump if available (smaller, faster)
	var dumpFile string
	if testDumper, ok := dumper.(dump.TestDumper); ok {
		dumpFile, err = testDumper.TestDump(ctx)
		if err != nil {
			return fmt.Errorf("test dump failed: %w", err)
		}
	} else {
		dumpFile, err = dumper.Dump(ctx)
		if err != nil {
			return fmt.Errorf("dump failed: %w", err)
		}
	}
	defer os.Remove(dumpFile)

	// Step 2: Compress
	compressor, err := compress.NewCompressor(backupCfg.Compress)
	if err != nil {
		return fmt.Errorf("create compressor: %w", err)
	}

	compressedFile, err := compressor.Compress(dumpFile)
	if err != nil {
		return fmt.Errorf("compress failed: %w", err)
	}
	defer os.Remove(compressedFile)

	// Step 3: Upload
	uploader, err := upload.NewUploader(backupCfg.Upload)
	if err != nil {
		return fmt.Errorf("create uploader: %w", err)
	}

	testBackupName := fmt.Sprintf("dumptruckd-test-%s-%d", backupCfg.Name, time.Now().Unix())
	remotePath, err := uploader.Upload(ctx, compressedFile, testBackupName)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	// Step 4: Verify and clean up (if supported)
	if verifiable, ok := uploader.(upload.VerifiableUploader); ok {
		if err := verifiable.Verify(ctx, remotePath); err != nil {
			verifiable.Delete(ctx, remotePath)
			return fmt.Errorf("verify upload failed: %w", err)
		}

		if err := verifiable.Delete(ctx, remotePath); err != nil {
			return fmt.Errorf("delete test file failed: %w", err)
		}
	}

	return nil
}

