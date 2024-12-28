package genai

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/generative-ai-go/genai"
)

const (
	testDir     = "synapse_files"
	testFile    = "test_file"
	testFileExt = "test_file.txt"
	testContent = "test content"
)

func TestMain(m *testing.M) {
	// Setup
	os.MkdirAll(testDir, os.ModePerm)
	// Run tests
	code := m.Run()
	// Cleanup
	os.RemoveAll(testDir)
	os.Exit(code)
}

func TestFileOperations(t *testing.T) {
	tests := []struct {
		name    string
		op      string
		file    string
		content string
		wantErr bool
		errMsg  string
	}{
		// Create operations
		{
			name:    "create valid file",
			op:      "create_file",
			file:    testFile,
			content: testContent,
			wantErr: false,
		},
		{
			name:    "create with empty filename",
			op:      "create_file",
			file:    "",
			content: testContent,
			wantErr: true,
			errMsg:  "invalid or missing file_name argument",
		},
		{
			name:    "create with empty content",
			op:      "create_file",
			file:    testFile,
			content: "",
			wantErr: true,
			errMsg:  "invalid or missing file_content argument",
		},

		// Read operations
		{
			name:    "read existing file",
			op:      "read_file",
			file:    testFileExt,
			content: testContent,
			wantErr: false,
		},
		{
			name:    "read non-existent file",
			op:      "read_file",
			file:    "nonexistent.txt",
			wantErr: true,
			errMsg:  "failed to read file",
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Cleanup before each test
			if tt.op == "create_file" {
				os.RemoveAll(testDir)
				os.MkdirAll(testDir, os.ModePerm)
			}

			// Setup for read tests
			if tt.op == "read_file" && !tt.wantErr {
				setupFile := filepath.Join(testDir, tt.file)
				err := os.WriteFile(setupFile, []byte(tt.content), 0644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			}

			funCall := genai.FunctionCall{
				Name: tt.op,
				Args: map[string]any{
					"file_name": tt.file,
				},
			}

			if tt.op == "create_file" {
				funCall.Args["file_content"] = tt.content
			}

			toolFunc, err := getTool(funCall.Name)
			if err != nil {
				t.Fatalf("getTool error: %v", err)
			}

			result, err := toolFunc(ctx, funCall)

			// Error checking
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Success case validation
			if tt.op == "create_file" {
				if !strings.Contains(result, "File created successfully at") {
					t.Errorf("unexpected success message: %s", result)
				}

				// Verify file content
				content, err := os.ReadFile(filepath.Join(testDir, tt.file+".txt"))
				if err != nil {
					t.Fatalf("failed to read created file: %v", err)
				}
				if string(content) != tt.content {
					t.Errorf("file content = %q, want %q", string(content), tt.content)
				}
			}

			if tt.op == "read_file" && result != tt.content {
				t.Errorf("got content = %q, want %q", result, tt.content)
			}
		})
	}
}

func TestCreateAndReadSequence(t *testing.T) {
	ctx := context.Background()

	// Clean start
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, os.ModePerm)

	// Create file
	createCall := genai.FunctionCall{
		Name: "create_file",
		Args: map[string]any{
			"file_name":    testFile,
			"file_content": testContent,
		},
	}

	createTool, _ := getTool(createCall.Name)
	result, err := createTool(ctx, createCall)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if !strings.Contains(result, "File created successfully at") {
		t.Errorf("unexpected create result: %s", result)
	}

	// Read file
	readCall := genai.FunctionCall{
		Name: "read_file",
		Args: map[string]any{
			"file_name": testFileExt,
		},
	}

	readTool, _ := getTool(readCall.Name)
	content, err := readTool(ctx, readCall)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if content != testContent {
		t.Errorf("got content = %q, want %q", content, testContent)
	}
}

func TestCleanupService(t *testing.T) {
	testDir := "test_synapse_files"
	if err := os.MkdirAll(testDir, os.ModePerm); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	testFiles := map[string]time.Duration{
		"old.txt":    -2 * time.Hour,
		"new.txt":    -30 * time.Minute,
		"oldest.txt": -3 * time.Hour,
		"newest.txt": -5 * time.Minute,
	}

	for name, age := range testFiles {
		filePath := filepath.Join(testDir, name)
		if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", name, err)
		}
		modTime := time.Now().Add(age)
		if err := os.Chtimes(filePath, modTime, modTime); err != nil {
			t.Fatalf("Failed to set modification time for %s: %v", name, err)
		}
	}

	// Create and start cleanup service
	cleanup := NewCleanupService(testDir)
	cleanup.Start()

	// Trigger immediate cleanup instead of waiting
	if err := cleanup.CleanupNow(); err != nil {
		t.Fatalf("Failed to perform cleanup: %v", err)
	}

	cleanup.Stop()

	files, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	expectedFiles := 2 // new.txt and newest.txt
	if len(files) != expectedFiles {
		t.Errorf("Expected %d files, got %d", expectedFiles, len(files))
	}

	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			t.Fatalf("Failed to get file info: %v", err)
		}

		if info.ModTime().Before(time.Now().Add(-time.Hour)) {
			t.Errorf("File %s should have been deleted (age: %v)",
				file.Name(), time.Since(info.ModTime()))
		}
	}
}

func TestCleanupServiceEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "non-existent directory",
			test: func(t *testing.T) {
				cleanup := NewCleanupService("non_existent_dir")
				cleanup.Start()
				err := cleanup.CleanupNow() // TRINGGER immediate cleanup
				if err != nil {
					t.Errorf("Expected no error for non-existent directory, got: %v", err)
				}
				cleanup.Stop()
			},
		},
		{
			name: "empty directory",
			test: func(t *testing.T) {
				dir := "empty_test_dir"
				os.MkdirAll(dir, os.ModePerm)
				defer os.RemoveAll(dir)

				cleanup := NewCleanupService(dir)
				cleanup.Start()
				err := cleanup.CleanupNow() // TRIGGER immediate cleanup
				if err != nil {
					t.Errorf("Expected no error for empty directory, got: %v", err)
				}
				cleanup.Stop()
			},
		},
		{
			name: "multiple start/stop",
			test: func(t *testing.T) {
				cleanup := NewCleanupService("test_dir")
				cleanup.Start()
				cleanup.Start() // Should not panic
				cleanup.Stop()
				cleanup.Stop() // Should not panic
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t)
		})
	}
}
