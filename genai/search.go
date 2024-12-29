package genai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

type WebsiteContent struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Author      string `json:"author"`
	Content     string `json:"content"`
}

const maxWorkers = 4

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

	selectors = []string{
		"article", "main", "#content",
		".content", ".article-content",
		".post-content", ".content p",
		"[role='main']", "[role='article']",
		"p", "h1", "h2", "h3", "h4", "h5",
		"ul:not(nav ul)", "ol:not(nav ol)",
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

	websiteContentPool = sync.Pool{
		New: func() any {
			return &WebsiteContent{}
		},
	}

	contentBuilderPool = sync.Pool{
		New: func() any {
			return &strings.Builder{}
		},
	}
)

func webSearch(ctx context.Context, funCall genai.FunctionCall) (string, error) {
	query, ok := funCall.Args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("invalid or missing query argument")
	}

	extractWebsites, ok := funCall.Args["extract_websites"].(bool)
	if !ok {
		return "", fmt.Errorf("invalid or missing extract_websites argument")
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

	if extractWebsites {
		var links []string
		for _, v := range results {
			links = append(links, v.Link)
		}
		return extractWebsitesContent(links), nil
	}

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(results); err != nil {
		return "", fmt.Errorf("error marshaling results: %w", err)
	}

	return buf.String(), nil
}

func extractWebsitesContent(links []string) string {

	resultChan := make(chan *WebsiteContent, len(links))
	results := make([]WebsiteContent, 0, len(links))
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for _, link := range links {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(ctx context.Context, link string) {
			defer wg.Done()
			defer func() { <-semaphore }()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic processing %s: %v", link, r)
				}
			}()

			extractedWebsiteContent := extractWebsiteContent(ctx, link)
			resultChan <- extractedWebsiteContent

		}(ctx, link)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		results = append(results, *result)
	}

	resultsByte, err := json.Marshal(results)
	if err != nil {
		log.Println("Error in Marshaling the results: ", err)
		return "{}"
	}

	return string(resultsByte)
}

func trimContent(content string) string {
	if len(content) == 0 {
		return ""
	}

	words := strings.Fields(content)
	if len(words) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.Grow(len(content))

	sb.WriteString(words[0])

	for _, word := range words[1:] {
		sb.WriteByte(' ')
		sb.WriteString(word)
	}

	return sb.String()
}

func removeDuplicateContent(content string) string {
	if len(content) == 0 {
		return ""
	}

	seen := make(map[string]struct{})
	builder := strings.Builder{}
	builder.Grow(len(content))

	isFirst := true
	for _, line := range strings.Split(content, "\n") {
		if _, exits := seen[line]; !exits {
			seen[line] = struct{}{}
			if !isFirst {
				builder.WriteByte('\n')
			}
			builder.WriteString(line)
			isFirst = false
		}
	}

	return builder.String()
}

func extractWebsiteContent(ctx context.Context, websiteLink string) *WebsiteContent {
	websiteContent := websiteContentPool.Get().(*WebsiteContent)
	*websiteContent = WebsiteContent{
		URL: websiteLink,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", websiteLink, nil)
	if err != nil {
		fmt.Printf("error creating request: %v", err)
		return websiteContent
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	res, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("Failed to send req: ", err)
		return websiteContent
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		fmt.Printf("received non-200 status code: %d", res.StatusCode)
		return websiteContent
	}

	// bodyBuf := bufPool.Get().(*bytes.Buffer)
	// bodyBuf.Reset()
	// defer bufPool.Put(bodyBuf)
	//
	// _, err = io.CopyN(bodyBuf, res.Body, 10<<20) // 10 MB
	// if err != nil && err != io.EOF {
	// 	log.Printf("error reading response body: %v", err)
	// 	return websiteContent
	// }

	reader := io.LimitReader(res.Body, 10<<20) // 10 MB

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		fmt.Printf("error parsing document: %v", err)
		return websiteContent
	}

	builder := contentBuilderPool.Get().(*strings.Builder)
	builder.Reset()
	defer contentBuilderPool.Put(builder)

	websiteContent.Title = doc.Find("title").Text()
	websiteContent.Description = trimContent(doc.Find(`meta[name="description"]`).AttrOr("content", ""))
	websiteContent.Date = doc.Find("time").AttrOr("datetime", "")
	websiteContent.Author = trimContent(doc.Find(`meta[name="author"]`).AttrOr("content", ""))

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				builder.WriteString(trimContent(text))
				builder.WriteString("\n")
			}
		})
	}

	websiteContent.Content = removeDuplicateContent(builder.String())
	return websiteContent
}
