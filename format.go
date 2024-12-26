package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var (
	patterns = []struct {
		regex *regexp.Regexp
		html  interface{}
	}{
		{regexp.MustCompile("(?s)```([a-zA-Z]*)\\s*(.*?)\\s*```"), nil},
		{regexp.MustCompile("`([^`]+)`"), "<code>$1</code>"},
		{regexp.MustCompile(`\*\*(.*?)\*\*`), "<b>$1</b>"},
		{regexp.MustCompile(`__(.*?)__`), "<u>$1</u>"},
		{regexp.MustCompile(`_(.*?)_`), "<i>$1</i>"},
		{regexp.MustCompile(`~~(.*?)~~`), "<s>$1</s>"},
		{regexp.MustCompile(`\|\|(.*?)\|\|`), "<tg-spoiler>$1</tg-spoiler>"},
		{regexp.MustCompile(`\[(.*?)\]\((.*?)\)`), "<a href=\"$2\">$1</a>"},
		{regexp.MustCompile(`(?m)^\*\s+(.*)`), "• $1"}, // ?m -> makes `^` work per line not just start and end of the input
	}

	multipleNewlines = regexp.MustCompile(`\n{3,}`)
	lineSpaces       = regexp.MustCompile(`(?m)^[ \t]+|[ \t]+$`)

	builderPool = sync.Pool{
		New: func() interface{} {
			return new(strings.Builder)
		},
	}
)

func init() {
	patterns[0].html = func(matches []string) string {
		if len(matches) < 3 {
			return matches[0]
		}
		code := matches[2]
		if code == "" {
			return ""
		}
		return fmt.Sprintf("<pre><code>%s</code></pre>", code)
	}
}

func convertToTelegramHTML(text string) string {
	if text == "" {
		return ""
	}

	builder := builderPool.Get().(*strings.Builder)
	builder.Reset()
	defer builderPool.Put(builder)

	builder.Grow(len(text) * 2)

	text = escapeUserInputForHTML(text)

	for _, pattern := range patterns {
		switch replacement := pattern.html.(type) {
		case string:
			text = pattern.regex.ReplaceAllString(text, replacement)
		case func([]string) string:
			text = pattern.regex.ReplaceAllStringFunc(text, func(match string) string {
				groups := pattern.regex.FindStringSubmatch(match)
				return replacement(groups)
			})
		}
	}

	// text = cleanupText(text)

	return text
}

func escapeUserInputForHTML(text string) string {
	builder := builderPool.Get().(*strings.Builder)
	builder.Reset()
	builder.Grow(len(text) + len(text)/4)
	defer builderPool.Put(builder)

	for _, r := range text {
		switch r {
		case '&':
			builder.WriteString("&amp;")
		case '<':
			builder.WriteString("&lt;")
		case '>':
			builder.WriteString("&gt;")
		default:
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

// func cleanupText(text string) string {
// 	text = multipleNewlines.ReplaceAllString(text, "\n\n")
// 	text = lineSpaces.ReplaceAllString(text, "")
// 	return strings.TrimSpace(text)
// }
