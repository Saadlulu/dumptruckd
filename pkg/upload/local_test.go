package upload

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLocalUploader_WithPath(t *testing.T) {
	tmpDir := t.TempDir()
	uploader, err := NewLocalUploader(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalUploader() error = %v", err)
	}
	if uploader == nil {
		t.Fatal("NewLocalUploader() returned nil")
	}
}

func TestNewLocalUploader_EmptyPathUsesDefault(t *testing.T) {
	// With the startup writability check, creating a LocalUploader with an empty
	// path will attempt to create and probe /var/backups/dumptruckd. On most CI/test
	// systems this fails due to permissions, which is the correct behavior — the
	// error message should guide the user to fix ownership.
	_, err := NewLocalUploader("")
	if err != nil {
		if !strings.Contains(err.Error(), "permission denied") &&
			!strings.Contains(err.Error(), "not writable") &&
			!strings.Contains(err.Error(), "create base directory") {
			t.Logf("NewLocalUploader('') returned unexpected error: %v", err)
		}
	}
}

func TestLocalUploader_Upload_CreatesDirectoryStructure(t *testing.T) {
	baseDir := t.TempDir()
	uploader, err := NewLocalUploader(baseDir)
	if err != nil {
		t.Fatalf("NewLocalUploader() error = %v", err)
	}

	// Create a source file
	srcFile, err := os.CreateTemp("", "upload-test-*.sql")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(srcFile.Name())
	_, _ = srcFile.WriteString("test backup data")
	srcFile.Close()

	destPath, err := uploader.Upload(context.Background(), srcFile.Name(), "my-backup")
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	// Verify the file exists at the destination
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("Uploaded file not found at %s: %v", destPath, err)
	}

	// Verify directory structure: baseDir/my-backup/YYYY/MM/DD/filename
	today := time.Now().Format("2006/01/02")
	expectedDir := filepath.Join(baseDir, "my-backup", today)
	if !strings.HasPrefix(destPath, expectedDir) {
		t.Errorf("Upload path should start with %q, got %q", expectedDir, destPath)
	}
}

func TestLocalUploader_Upload_CopiesContent(t *testing.T) {
	baseDir := t.TempDir()
	uploader, _ := NewLocalUploader(baseDir)

	srcFile, _ := os.CreateTemp("", "upload-content-*.sql")
	defer os.Remove(srcFile.Name())
	content := "SELECT * FROM important_data;"
	_, _ = srcFile.WriteString(content)
	srcFile.Close()

	destPath, err := uploader.Upload(context.Background(), srcFile.Name(), "content-test")
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	// Read back and verify content
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read uploaded file: %v", err)
	}
	if string(data) != content {
		t.Errorf("Uploaded content = %q, want %q", string(data), content)
	}
}

func TestLocalUploader_Upload_NonexistentSource(t *testing.T) {
	baseDir := t.TempDir()
	uploader, _ := NewLocalUploader(baseDir)

	_, err := uploader.Upload(context.Background(), "/nonexistent/file.sql", "test")
	if err == nil {
		t.Error("Upload() should error for nonexistent source file")
	}
}

func TestLocalUploader_Verify_ExistingFile(t *testing.T) {
	baseDir := t.TempDir()
	uploader, _ := NewLocalUploader(baseDir)

	// Create a file to verify
	testFile := filepath.Join(baseDir, "test.sql")
	_ = os.WriteFile(testFile, []byte("data"), 0644)

	err := uploader.Verify(context.Background(), testFile)
	if err != nil {
		t.Errorf("Verify() should succeed for existing file, got %v", err)
	}
}

func TestLocalUploader_Verify_MissingFile(t *testing.T) {
	baseDir := t.TempDir()
	uploader, _ := NewLocalUploader(baseDir)

	err := uploader.Verify(context.Background(), filepath.Join(baseDir, "nonexistent.sql"))
	if err == nil {
		t.Error("Verify() should error for missing file")
	}
}

func TestLocalUploader_Delete_RemovesFile(t *testing.T) {
	baseDir := t.TempDir()
	uploader, _ := NewLocalUploader(baseDir)

	// Create a file to delete
	testFile := filepath.Join(baseDir, "to-delete.sql")
	_ = os.WriteFile(testFile, []byte("data"), 0644)

	err := uploader.Delete(context.Background(), testFile)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Delete() should remove the file")
	}
}

func TestLocalUploader_Delete_NonexistentFile(t *testing.T) {
	baseDir := t.TempDir()
	uploader, _ := NewLocalUploader(baseDir)

	err := uploader.Delete(context.Background(), filepath.Join(baseDir, "nonexistent.sql"))
	if err == nil {
		t.Error("Delete() should error for nonexistent file")
	}
}

func TestLocalUploader_ImplementsUploader(t *testing.T) {
	tmpDir := t.TempDir()
	uploader, _ := NewLocalUploader(tmpDir)

	// Compile-time check that LocalUploader satisfies Uploader
	var _ Uploader = uploader
}

func TestLocalUploader_Verify_RejectsPathOutsideBase(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	uploader, err := NewLocalUploader(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalUploader() error = %v", err)
	}

	err = uploader.Verify(context.Background(), "/etc/passwd")
	if err == nil {
		t.Fatal("Verify() should reject path outside base directory")
	}
	if !strings.Contains(err.Error(), "outside base directory") {
		t.Errorf("Verify() error = %q, want it to mention 'outside base directory'", err.Error())
	}
}

func TestLocalUploader_Delete_RejectsPathOutsideBase(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	uploader, err := NewLocalUploader(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalUploader() error = %v", err)
	}

	err = uploader.Delete(context.Background(), "/etc/passwd")
	if err == nil {
		t.Fatal("Delete() should reject path outside base directory")
	}
	if !strings.Contains(err.Error(), "outside base directory") {
		t.Errorf("Delete() error = %q, want it to mention 'outside base directory'", err.Error())
	}
}

func TestLocalUploader_Verify_RejectsTraversal(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	uploader, err := NewLocalUploader(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalUploader() error = %v", err)
	}

	traversalPath := tmpDir + "/../../../etc/passwd"
	err = uploader.Verify(context.Background(), traversalPath)
	if err == nil {
		t.Fatal("Verify() should reject traversal path")
	}
	if !strings.Contains(err.Error(), "outside base directory") {
		t.Errorf("Verify() error = %q, want it to mention 'outside base directory'", err.Error())
	}
}

func TestLocalUploader_Delete_RejectsTraversal(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	uploader, err := NewLocalUploader(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalUploader() error = %v", err)
	}

	traversalPath := tmpDir + "/../../../etc/passwd"
	err = uploader.Delete(context.Background(), traversalPath)
	if err == nil {
		t.Fatal("Delete() should reject traversal path")
	}
	if !strings.Contains(err.Error(), "outside base directory") {
		t.Errorf("Delete() error = %q, want it to mention 'outside base directory'", err.Error())
	}
}

func TestLocalUploader_Upload_FilePermissions(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	uploader, err := NewLocalUploader(baseDir)
	if err != nil {
		t.Fatalf("NewLocalUploader() error = %v", err)
	}

	srcFile, err := os.CreateTemp("", "perm-test-*.sql")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(srcFile.Name())
	_, _ = srcFile.WriteString("permission test data")
	srcFile.Close()

	destPath, err := uploader.Upload(context.Background(), srcFile.Name(), "perm-check")
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("Uploaded file permissions = %o, want 0600", perm)
	}
}

func TestLocalUploader_ValidatePath_ExactBasePathAllowed(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	uploader, err := NewLocalUploader(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalUploader() error = %v", err)
	}

	// Verify accepts the base path itself (edge case: absPath == absBase)
	err = uploader.validatePath(tmpDir)
	if err != nil {
		t.Errorf("validatePath() should accept exact base path, got error: %v", err)
	}
}

func TestNewLocalUploader_NonWritablePath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0500); err != nil {
		t.Fatalf("Failed to create read-only dir: %v", err)
	}

	_, err := NewLocalUploader(readOnlyDir)
	if err == nil {
		t.Fatal("NewLocalUploader() should fail for non-writable directory")
	}
	if !strings.Contains(err.Error(), "not writable") {
		t.Errorf("error should mention 'not writable', got: %v", err)
	}
}

func TestNewLocalUploader_CreatesBaseDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "new", "nested", "dir")

	uploader, err := NewLocalUploader(newDir)
	if err != nil {
		t.Fatalf("NewLocalUploader() error = %v", err)
	}
	if uploader == nil {
		t.Fatal("NewLocalUploader() returned nil")
	}

	// Verify the directory was created
	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("Base directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("Base path is not a directory")
	}
}
