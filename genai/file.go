package genai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/generative-ai-go/genai"
)

func createFile(ctx context.Context, funCall genai.FunctionCall) (string, error) {

	// 1. Get `fileName` from args
	fileName, ok := funCall.Args["file_name"].(string)
	if !ok || fileName == "" {
		return "", fmt.Errorf("invalid or missing file_name argument")
	}

	// 2. Get `fileContent` from args
	fileContent, ok := funCall.Args["file_content"].(string)
	if !ok || fileContent == "" {
		return "", fmt.Errorf("invalid or missing file_content argument")
	}

	// 3. Get working dir
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %v", err)
	}

	// 4. Create working directory for files
	fileDir := filepath.Join(dir, "synapse_files")
	err = os.MkdirAll(fileDir, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

	// 5. Construct file path
	filePath := filepath.Join(fileDir, fileName)
	filePath = filePath + ".txt"

	// 6. Create file
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// 7. Write content to file
	_, err = file.WriteString(fileContent)
	if err != nil {
		return "", fmt.Errorf("failed to write to file: %v", err)
	}

	return fmt.Sprintf("File created successfully at %s", filePath), nil
}

func readFile(ctx context.Context, fc genai.FunctionCall) (string, error) {
	filename, ok := fc.Args["file_name"].(string)
	if !ok {
		return "", fmt.Errorf("invalid or missing file_name argument")
	}

	filePath := "synapse_files" + "/" + filename

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}
	return string(content), nil
}
