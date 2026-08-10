package unfurl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Preview — OG-карточка ссылки в сообщении.
type Preview struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image"`
}

var urlRe = regexp.MustCompile(`https?://[^\s<>"']+`)

// ExtractURLs возвращает уникальные URL, встречающиеся в тексте.
func ExtractURLs(text string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, m := range urlRe.FindAllString(text, -1) {
		u := strings.TrimRight(m, ".,;:!?)")
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// Fetch скачивает страницу и извлекает OG-метаданные.
func Fetch(ctx context.Context, rawURL string) (*Preview, error) {
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(cctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "HazeLinkPreview/1.0 (+https://github.com/cotton-msg)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	p := &Preview{URL: rawURL}
	walk(doc, p)
	if p.Title == "" && p.Description == "" && p.Image == "" {
		return nil, fmt.Errorf("no og metadata found")
	}
	return p, nil
}

// walk обходит дерево HTML, собирая og: и title.
func walk(n *html.Node, p *Preview) {
	if n.Type == html.ElementNode && n.Data == "meta" {
		var property, name, content string
		for _, a := range n.Attr {
			switch a.Key {
			case "property", "name":
				property = strings.ToLower(a.Val)
				name = a.Val
			case "content":
				content = strings.TrimSpace(a.Val)
			}
		}
		prop := property
		if prop == "" {
			prop = name
		}
		switch prop {
		case "og:title":
			if content != "" {
				p.Title = content
			}
		case "og:description", "description":
			if p.Description == "" {
				p.Description = content
			}
		case "og:image", "twitter:image":
			if p.Image == "" {
				p.Image = content
			}
		}
	}
	if n.Type == html.ElementNode && n.Data == "title" && p.Title == "" {
		if n.FirstChild != nil {
			p.Title = strings.TrimSpace(n.FirstChild.Data)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, p)
	}
}
