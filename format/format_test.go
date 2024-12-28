package format

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func TestConvertToTelegramHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Plain text",
			input:    "Hello, world!",
			expected: "Hello, world!",
		},
		{
			name:     "Bold text",
			input:    "Hello **bold** text",
			expected: "Hello <b>bold</b> text",
		},
		{
			name:     "Italic text",
			input:    "Hello _italic_ text",
			expected: "Hello <i>italic</i> text",
		},
		{
			name:     "Underline text",
			input:    "Hello __underline__ text",
			expected: "Hello <u>underline</u> text",
		},
		{
			name:     "Strikethrough text",
			input:    "Hello ~~strikethrough~~ text",
			expected: "Hello <s>strikethrough</s> text",
		},
		{
			name:     "Spoiler text",
			input:    "Hello ||spoiler|| text",
			expected: "Hello <tg-spoiler>spoiler</tg-spoiler> text",
		},
		{
			name:     "Inline code",
			input:    "Hello `inline code` text",
			expected: "Hello <code>inline code</code> text",
		},
		{
			name:     "Code block without language",
			input:    "```\nSimple code block\n```",
			expected: "<pre><code>Simple code block</code></pre>",
		},
		{
			name:     "Code block with language",
			input:    "```python\nprint('Hello')\n```",
			expected: "<pre><code>print('Hello')</code></pre>",
		},
		{
			name:     "Link",
			input:    "[Example](https://example.com)",
			expected: "<a href=\"https://example.com\">Example</a>",
		},
		{
			name:     "Bullet points",
			input:    "* First\n* Second",
			expected: "• First\n• Second",
		},
		{
			name:     "Combined formatting",
			input:    "**Bold** _italic_ `code` ||spoiler||",
			expected: "<b>Bold</b> <i>italic</i> <code>code</code> <tg-spoiler>spoiler</tg-spoiler>",
		},
		{
			name:     "HTML escape",
			input:    "<div>&test</div>",
			expected: "&lt;div&gt;&amp;test&lt;/div&gt;",
		},
		{
			name:     "Multiple newlines",
			input:    "Line1\n\n\n\nLine2",
			expected: "Line1\n\nLine2",
		},
		// {
		// 	name:     "Code block without newlines",
		// 	input:    "```const x = 1;```",
		// 	expected: "<pre><code>const x = 1;</code></pre>",
		// },
		{
			name:     "Code block with multiple lines",
			input:    "```\nline1\nline2\nline3\n```",
			expected: "<pre><code>line1\nline2\nline3</code></pre>",
		},
		// {
		// 	name:     "Code block with language and no initial newline",
		// 	input:    "```goprintln('Hello')```",
		// 	expected: "<pre><code>println('Hello')</code></pre>",
		// },
		{
			name:     "Empty code block",
			input:    "```\n```",
			expected: "",
		},
		{
			name:     "Code block with spaces",
			input:    "```   \ncode\n   ```",
			expected: "<pre><code>code</code></pre>",
		},
		// {
		// 	name:     "Nested formatting in code block",
		// 	input:    "```\n**bold** _italic_\n```",
		// 	expected: "<pre><code>**bold** _italic_</code></pre>",
		// },
		{
			name: "Complex example",
			input: `* **Bold item**
* _Italic item_
* Code example:
` + "```python\nprint('test')\n```" + `
* [Link](https://test.com)`,
			expected: `• <b>Bold item</b>
• <i>Italic item</i>
• Code example:
<pre><code>print('test')</code></pre>
• <a href="https://test.com">Link</a>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToTelegramHTML(tt.input)
			if result != tt.expected {
				t.Errorf("\nInput:\n%s\nExpected:\n%s\nGot:\n%s",
					tt.input, tt.expected, result)
			}
		})
	}
}

func TestEscapeUserInputForHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Plain text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "HTML chars",
			input:    "<div>&Hello</div>",
			expected: "&lt;div&gt;&amp;Hello&lt;/div&gt;",
		},
		{
			name:     "Multiple special chars",
			input:    "< > & < >",
			expected: "&lt; &gt; &amp; &lt; &gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeUserInputForHTML(tt.input)
			if result != tt.expected {
				t.Errorf("\nInput: %s\nExpected: %s\nGot: %s",
					tt.input, tt.expected, result)
			}
		})
	}
}

// func TestCleanupText(t *testing.T) {
// 	tests := []struct {
// 		name     string
// 		input    string
// 		expected string
// 	}{
// 		{
// 			name:     "Normal text",
// 			input:    "Hello world",
// 			expected: "Hello world",
// 		},
// 		{
// 			name:     "Multiple newlines",
// 			input:    "Line1\n\n\n\nLine2",
// 			expected: "Line1\n\nLine2",
// 		},
// 		{
// 			name:     "Trim spaces",
// 			input:    "  Hello  \n  World  ",
// 			expected: "Hello\nWorld",
// 		},
// 		{
// 			name:     "Mixed whitespace",
// 			input:    " \t Hello \t \n \t World \t ",
// 			expected: "Hello\nWorld",
// 		},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result := cleanupText(tt.input)
// 			if result != tt.expected {
// 				t.Errorf("\nInput: %s\nExpected: %s\nGot: %s",
// 					tt.input, tt.expected, result)
// 			}
// 		})
// 	}
// }

func BenchmarkConvertToTelegramHTML(b *testing.B) {
	// Different test cases for benchmarking
	cases := []struct {
		name string
		text string
	}{
		{
			name: "Simple",
			text: "Hello **bold** _italic_",
		},
		{
			name: "Complex",
			text: `# Complex Document
**Bold Text** with _italic_ and __underline__
* List item 1
* List item 2

` + "```python\ndef hello():\n    print('Hello, World!')\n```" + `

[Link](https://example.com)
||spoiler text||
~~strikethrough~~`,
		},
		{
			name: "Code Heavy",
			text: "```go\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n```\n`inline code`\n```python\nprint('hi')\n```",
		},
		{
			name: "List Heavy",
			text: "* Item 1\n* Item 2\n* Item 3\n* Item 4\n* Item 5\n* Item 6\n* Item 7\n* Item 8\n* Item 9\n* Item 10",
		},
		{
			name: "Mixed Formatting",
			text: "**Bold** _Italic_ __Underline__ ~~Strike~~ ||Spoiler|| `Code` [Link](url)",
		},
	}

	// Run benchmarks for each case
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			// Reset timer for each test case
			b.ResetTimer()

			// Run the conversion b.N times
			for i := 0; i < b.N; i++ {
				ConvertToTelegramHTML(tc.text)
			}
		})
	}
}

// Benchmark individual components
func BenchmarkEscapeUserInputForHTML(b *testing.B) {
	text := "Hello <div>Test & More</div>"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		escapeUserInputForHTML(text)
	}
}

// func BenchmarkCleanupText(b *testing.B) {
// 	text := "Line 1\n\n\n\nLine 2  \n  Line 3  \n\n\n\nLine 4"
// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		cleanupText(text)
// 	}
// }

// Benchmark with different input sizes
func BenchmarkConvertToTelegramHTML_InputSize(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			// Generate test input of specific size
			text := generateTestInput(size)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ConvertToTelegramHTML(text)
			}
		})
	}
}

// Helper function to generate test input
func generateTestInput(size int) string {
	patterns := []string{
		"**Bold**",
		"_Italic_",
		"__Underline__",
		"~~Strike~~",
		"||Spoiler||",
		"`Code`",
		"* List item",
		"[Link](url)",
		"```\ncode block\n```",
	}

	var builder strings.Builder
	for builder.Len() < size {
		pattern := patterns[rand.Intn(len(patterns))]
		builder.WriteString(pattern)
		builder.WriteString(" ")
	}

	return builder.String()
}

// Benchmark memory allocations
func BenchmarkConvertToTelegramHTML_Allocs(b *testing.B) {
	text := "**Bold** _Italic_ `Code` ||Spoiler||"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ConvertToTelegramHTML(text)
	}
}
