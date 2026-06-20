package feed

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/hjosugi/signal-garden/internal/config"
	"github.com/hjosugi/signal-garden/internal/model"
)

const maxFeedBytes = 5 << 20

var (
	tagPattern        = regexp.MustCompile(`(?s)<[^>]*>`)
	whitespacePattern = regexp.MustCompile(`\s+`)
)

type Fetcher struct {
	client *http.Client
	now    func() time.Time
}

func NewFetcher(timeout time.Duration) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return validateHTTPURL(req.URL)
			},
		},
		now: func() time.Time { return time.Now().UTC() },
	}
}

type Result struct {
	Items      []model.Item
	StatusCode int
}

func (f *Fetcher) Fetch(ctx context.Context, cfg config.Feed) (Result, error) {
	parsed, err := url.Parse(cfg.URL)
	if err != nil {
		return Result{}, fmt.Errorf("invalid feed URL: %w", err)
	}
	if err := validateHTTPURL(parsed); err != nil {
		return Result{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", "signal-garden/0.1 (+https://github.com/hjosugi/signal-garden)")
	req.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/xml, text/xml;q=0.9, */*;q=0.1")

	resp, err := f.client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{StatusCode: resp.StatusCode}, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return Result{StatusCode: resp.StatusCode}, err
	}
	if len(body) > maxFeedBytes {
		return Result{StatusCode: resp.StatusCode}, fmt.Errorf("feed exceeds %d bytes", maxFeedBytes)
	}
	items, err := Parse(body, parsed, cfg, f.now())
	if err != nil {
		return Result{StatusCode: resp.StatusCode}, err
	}
	return Result{Items: items, StatusCode: resp.StatusCode}, nil
}

func Parse(data []byte, base *url.URL, cfg config.Feed, now time.Time) ([]model.Item, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("empty XML document")
		}
		if err != nil {
			return nil, fmt.Errorf("read XML root: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch strings.ToLower(start.Name.Local) {
		case "rss":
			var doc rssDocument
			if err := decoder.DecodeElement(&doc, &start); err != nil {
				return nil, fmt.Errorf("parse RSS: %w", err)
			}
			return normalizeRSS(doc.Channel.Items, base, cfg, now), nil
		case "rdf":
			var doc rdfDocument
			if err := decoder.DecodeElement(&doc, &start); err != nil {
				return nil, fmt.Errorf("parse RDF/RSS: %w", err)
			}
			return normalizeRSS(doc.Items, base, cfg, now), nil
		case "feed":
			var doc atomDocument
			if err := decoder.DecodeElement(&doc, &start); err != nil {
				return nil, fmt.Errorf("parse Atom: %w", err)
			}
			return normalizeAtom(doc.Entries, base, cfg, now), nil
		default:
			return nil, fmt.Errorf("unsupported feed root <%s>", start.Name.Local)
		}
	}
}

type rssDocument struct {
	Channel rssChannel `xml:"channel"`
}

type rdfDocument struct {
	Items []rssItem `xml:"item"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssGUID struct {
	Value string `xml:",chardata"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	GUID        rssGUID  `xml:"guid"`
	Description string   `xml:"description"`
	Content     string   `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	Author      string   `xml:"author"`
	Creator     string   `xml:"http://purl.org/dc/elements/1.1/ creator"`
	PubDate     string   `xml:"pubDate"`
	Date        string   `xml:"http://purl.org/dc/elements/1.1/ date"`
	Categories  []string `xml:"category"`
}

type atomDocument struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title      string      `xml:"title"`
	ID         string      `xml:"id"`
	Links      []atomLink  `xml:"link"`
	Summary    string      `xml:"summary"`
	Content    atomContent `xml:"content"`
	Published  string      `xml:"published"`
	Updated    string      `xml:"updated"`
	Authors    []atomName  `xml:"author"`
	Categories []atomCat   `xml:"category"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type atomContent struct {
	Value string `xml:",innerxml"`
}

type atomName struct {
	Name string `xml:"name"`
}

type atomCat struct {
	Term string `xml:"term,attr"`
}

func normalizeRSS(entries []rssItem, base *url.URL, cfg config.Feed, now time.Time) []model.Item {
	items := make([]model.Item, 0, len(entries))
	for _, entry := range entries {
		rawContent := firstNonEmpty(entry.Content, entry.Description)
		content := cleanText(rawContent)
		link := resolveURL(base, strings.TrimSpace(entry.Link))
		rawID := firstNonEmpty(strings.TrimSpace(entry.GUID.Value), link, strings.TrimSpace(entry.Title))
		if rawID == "" {
			continue
		}
		published := parseDate(firstNonEmpty(entry.PubDate, entry.Date))
		if published.IsZero() {
			published = now
		}
		items = append(items, model.Item{
			ID:          stableID(cfg.ID, rawID),
			SourceID:    cfg.ID,
			SourceName:  cfg.Name,
			SourceKind:  cfg.Kind,
			Title:       cleanText(entry.Title),
			URL:         link,
			Author:      cleanText(firstNonEmpty(entry.Creator, entry.Author)),
			Summary:     summarize(content),
			Content:     truncateRunes(content, 8000),
			PublishedAt: published.UTC(),
			CollectedAt: now.UTC(),
			Tags:        mergeTags(cfg.Tags, entry.Categories),
		})
	}
	return items
}

func normalizeAtom(entries []atomEntry, base *url.URL, cfg config.Feed, now time.Time) []model.Item {
	items := make([]model.Item, 0, len(entries))
	for _, entry := range entries {
		link := atomEntryURL(entry.Links, base)
		rawID := firstNonEmpty(strings.TrimSpace(entry.ID), link, strings.TrimSpace(entry.Title))
		if rawID == "" {
			continue
		}
		rawContent := firstNonEmpty(entry.Content.Value, entry.Summary)
		content := cleanText(rawContent)
		published := parseDate(firstNonEmpty(entry.Published, entry.Updated))
		if published.IsZero() {
			published = now
		}
		author := ""
		if len(entry.Authors) > 0 {
			author = cleanText(entry.Authors[0].Name)
		}
		categories := make([]string, 0, len(entry.Categories))
		for _, category := range entry.Categories {
			categories = append(categories, category.Term)
		}
		items = append(items, model.Item{
			ID:          stableID(cfg.ID, rawID),
			SourceID:    cfg.ID,
			SourceName:  cfg.Name,
			SourceKind:  cfg.Kind,
			Title:       cleanText(entry.Title),
			URL:         link,
			Author:      author,
			Summary:     summarize(content),
			Content:     truncateRunes(content, 8000),
			PublishedAt: published.UTC(),
			CollectedAt: now.UTC(),
			Tags:        mergeTags(cfg.Tags, categories),
		})
	}
	return items
}

func atomEntryURL(links []atomLink, base *url.URL) string {
	for _, link := range links {
		if link.Rel == "alternate" || link.Rel == "" {
			if resolved := resolveURL(base, link.Href); resolved != "" {
				return resolved
			}
		}
	}
	for _, link := range links {
		if resolved := resolveURL(base, link.Href); resolved != "" {
			return resolved
		}
	}
	return ""
}

func validateHTTPURL(u *url.URL) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("URL must use http or https and include a host")
	}
	return nil
}

func resolveURL(base *url.URL, raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if err := validateHTTPURL(u); err != nil {
		return ""
	}
	return u.String()
}

func stableID(sourceID, rawID string) string {
	sum := sha256.Sum256([]byte(sourceID + "\x00" + rawID))
	return hex.EncodeToString(sum[:16])
}

func cleanText(raw string) string {
	if raw == "" {
		return ""
	}
	withoutTags := tagPattern.ReplaceAllString(raw, " ")
	decoded := html.UnescapeString(withoutTags)
	decoded = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, decoded)
	return strings.TrimSpace(whitespacePattern.ReplaceAllString(decoded, " "))
}

func summarize(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "No summary provided by the source."
	}
	return truncateRunes(content, 360)
}

func truncateRunes(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	cut := max
	for i := max; i > max-60 && i > 0; i-- {
		if unicode.IsSpace(runes[i-1]) || strings.ContainsRune("。.!?！？", runes[i-1]) {
			cut = i
			break
		}
	}
	return strings.TrimSpace(string(runes[:cut])) + "…"
}

func mergeTags(groups ...[]string) []string {
	set := map[string]struct{}{}
	for _, group := range groups {
		for _, tag := range group {
			tag = normalizeTag(tag)
			if tag != "" {
				set[tag] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for tag := range set {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func normalizeTag(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	tag = strings.ReplaceAll(tag, "_", "-")
	tag = whitespacePattern.ReplaceAllString(tag, "-")
	return strings.Trim(tag, "-/#")
}

func parseDate(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC850,
		time.ANSIC,
		"Mon, 02 Jan 2006 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
