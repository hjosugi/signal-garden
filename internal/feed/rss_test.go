package feed

import (
	"net/url"
	"testing"
	"time"

	"github.com/hjosugi/signal-garden/internal/config"
)

func TestParseRSS(t *testing.T) {
	xml := `<?xml version="1.0"?><rss version="2.0"><channel><item><title>Cloud release</title><link>/post</link><description><![CDATA[<p>New distributed database feature.</p>]]></description><pubDate>Fri, 19 Jun 2026 10:00:00 +0000</pubDate><category>Cloud</category></item></channel></rss>`
	base, _ := url.Parse("https://example.com/feed")
	items, err := Parse([]byte(xml), base, config.Feed{ID: "example", Name: "Example", Kind: "rss", Tags: []string{"official"}}, time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].URL != "https://example.com/post" {
		t.Fatalf("unexpected URL: %s", items[0].URL)
	}
	if items[0].Summary != "New distributed database feature." {
		t.Fatalf("unexpected summary: %s", items[0].Summary)
	}
}

func TestParseAtom(t *testing.T) {
	xml := `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><entry><title>Typed systems</title><id>tag:example,1</id><link href="https://example.com/typed"/><summary>Strong types and simple operations.</summary><updated>2026-06-20T01:00:00Z</updated><author><name>Alice</name></author></entry></feed>`
	base, _ := url.Parse("https://example.com/feed")
	items, err := Parse([]byte(xml), base, config.Feed{ID: "atom", Name: "Atom", Kind: "atom"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Author != "Alice" {
		t.Fatalf("unexpected item: %#v", items)
	}
}
