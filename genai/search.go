package genai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/generative-ai-go/genai"
)

type SearchResult struct {
	Title    string `json:"title"`
	Link     string `json:"link"`
	Snippet  string `json:"snippet"`
	Position int    `json:"position"`
}

func webSearch(ctx context.Context, funCall genai.FunctionCall) (string, error) {
	query, ok := funCall.Args["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("invalid or missing file_name argument")
	}
	url := fmt.Sprintf("https://google.com/search?q=%s&gl&hl=en", url.QueryEscape(query))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error performing request: %w", err)
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", fmt.Errorf("error parsing document: %w", err)
	}

	var results []SearchResult
	c := 0
	doc.Find("div.g").Each(func(i int, s *goquery.Selection) {
		title := s.Find("h3").First().Text()
		link, _ := s.Find("a").First().Attr("href")
		snippet := s.Find(".VwiC3b").First().Text()

		result := SearchResult{
			Title:    title,
			Link:     link,
			Snippet:  snippet,
			Position: c + 1,
		}
		results = append(results, result)
		c++
	})

	finalResult, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("error marshaling results: %w", err)
	}
	return string(finalResult), nil
}
