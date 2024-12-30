package genai

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
)

var availableTools = map[string]func(ctx context.Context, args genai.FunctionCall) (string, error){
	"create_file":      createFile,
	"read_file":        readFile,
	"web_search":       webSearch,
	"extract_websites": extractWebPagesContent,
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
		// {
		// 	Name:        "read_file",
		// 	Description: "Read content from a file",
		// 	Parameters: &genai.Schema{
		// 		Type: genai.TypeObject,
		// 		Properties: map[string]*genai.Schema{
		// 			"file_name": {
		// 				Type:        genai.TypeString,
		// 				Description: "Name of the file to read",
		// 			},
		// 		},
		// 		Required: []string{"file_name"},
		// 	},
		// },
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
		{
			Name:        "web_search",
			Description: "Perform a web search and optionally extract data from top search results.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"query": {
						Type:        genai.TypeString,
						Description: "The search query to execute on the web (returns top search results).",
					},
					"extract_websites": {
						Type:        genai.TypeBoolean,
						Description: "If true, data will be extracted from each top search result.",
					},
				},
				Required: []string{"query", "extract_websites"},
			},
		},
		{
			Name:        "extract_websites",
			Description: "Extract data from given links.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"links": {
						Type:        genai.TypeArray,
						Description: "An array of links from which data needs to be extracted.",
						Items: &genai.Schema{
							Type:        genai.TypeString,
							Description: "link to scrape",
						},
					},
				},
				Required: []string{"links"},
			},
		},
	},
}
