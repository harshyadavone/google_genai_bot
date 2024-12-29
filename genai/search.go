package genai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/generative-ai-go/genai"
)

type SearchResult struct {
	Title    string `json:"title,omitempty"`
	Link     string `json:"link,omitempty"`
	Snippet  string `json:"snippet,omitempty"`
	Position int    `json:"position,omitempty"`
}

var (
	httpClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     30 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   3 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
		Timeout: 10 * time.Second,
	}

	resultsPool = sync.Pool{
		New: func() any {
			return make([]SearchResult, 0, 10)
		},
	}

	bufPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, 1024))
		},
	}
)

func webSearch(ctx context.Context, funCall genai.FunctionCall) (string, error) {
	query, ok := funCall.Args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("invalid or missing file_name argument")
	}

	url := fmt.Sprintf("https://google.com/search?q=%s&gl&hl=en", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	res, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch results for query '%s' : %w", query, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 status code: %d", res.StatusCode)
	}

	bodyReader := bufio.NewReader(res.Body)

	doc, err := goquery.NewDocumentFromReader(bodyReader)
	if err != nil {
		return "", fmt.Errorf("error parsing document: %w", err)
	}

	results := resultsPool.Get().([]SearchResult)
	results = results[:0]
	defer resultsPool.Put(results)
	doc.Find("div.g").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find("h3").First().Text())
		link, exists := s.Find("a").First().Attr("href")
		if !exists {
			return
		}
		link = strings.TrimSpace(link)
		snippet := strings.TrimSpace(s.Find(".VwiC3b").First().Text())

		if title != "" && link != "" {
			results = append(results, SearchResult{
				Title:    title,
				Link:     link,
				Snippet:  snippet,
				Position: len(results) + 1,
			})
		}
	})

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(results); err != nil {
		return "", fmt.Errorf("error marshaling results: %w", err)
	}

	return buf.String(), nil
}
