package main

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
)

var availableTools = map[string]func(ctx context.Context, args genai.FunctionCall) (string, error){
	"create_file": createFile,
	"read_file":   readFile,
}

func getTool(name string) (func(ctx context.Context, args genai.FunctionCall) (string, error), error) {
	tool, ok := availableTools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found")
	}
	return tool, nil
}

var tools = &genai.Tool{
	FunctionDeclarations: []*genai.FunctionDeclaration{
		{
			Name:        "read_file",
			Description: "Read content from a file",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"file_name": {
						Type:        genai.TypeString,
						Description: "Name of the file to read",
					},
				},
				Required: []string{"file_name"},
			},
		},
		{
			Name:        "create_file",
			Description: "Creates a file for given content and filename",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"file_name": {
						Type:        genai.TypeString,
						Description: "file name without the extension for example : rust_book , i'll add extension later (.txt)",
					},
					"file_content": {
						Type:        genai.TypeString,
						Description: "File conten which will be written to file it should be string",
					},
				},
				Required: []string{"file_name", "file_content"},
			},
		},
	},
}
