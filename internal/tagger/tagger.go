package tagger

import (
	"sort"
	"strings"
)

type Rule struct {
	Tag      string
	Keywords []string
}

type Tagger struct {
	rules []Rule
}

func New() *Tagger {
	return &Tagger{rules: []Rule{
		{Tag: "google-cloud", Keywords: []string{"google cloud", "gcp", "bigquery", "cloud run", "spanner", "vertex ai"}},
		{Tag: "aws", Keywords: []string{"amazon web services", "aws", "lambda", "dynamodb", "s3", "eks", "bedrock"}},
		{Tag: "azure", Keywords: []string{"microsoft azure", "azure", "cosmos db", "aks"}},
		{Tag: "cloud", Keywords: []string{"cloud infrastructure", "cloud computing", "serverless", "kubernetes", "container"}},
		{Tag: "distributed-systems", Keywords: []string{"distributed system", "consensus", "replication", "partition", "raft", "paxos", "fault tolerance", "eventual consistency"}},
		{Tag: "database", Keywords: []string{"database", "postgresql", "mysql", "sqlite", "spanner", "dynamodb", "snowflake", "databricks", "vector database", "query engine"}},
		{Tag: "data-engineering", Keywords: []string{"data pipeline", "data engineering", "stream processing", "batch processing", "apache kafka", "apache flink", "spark", "warehouse", "lakehouse"}},
		{Tag: "ai-ml", Keywords: []string{"machine learning", "artificial intelligence", "llm", "large language model", "embedding", "inference", "transformer", "rag", "agent"}},
		{Tag: "security", Keywords: []string{"security", "vulnerability", "cve", "zero trust", "authentication", "authorization", "encryption"}},
		{Tag: "observability", Keywords: []string{"observability", "opentelemetry", "tracing", "metrics", "logging", "sre"}},
		{Tag: "developer-tools", Keywords: []string{"developer tools", "build system", "compiler", "runtime", "sdk", "api design", "devex", "developer experience"}},
		{Tag: "frontend", Keywords: []string{"frontend", "browser", "javascript", "typescript", "react", "vue", "svelte", "css"}},
		{Tag: "career", Keywords: []string{"engineering career", "interview", "staff engineer", "leadership", "engineering management"}},
		{Tag: "日本語", Keywords: []string{"日本語", "国内", "開発者向け"}},
	}}
}

func (t *Tagger) Apply(title, content string, seed []string) []string {
	text := strings.ToLower(title + "\n" + content)
	set := make(map[string]struct{}, len(seed)+6)
	for _, tag := range seed {
		tag = normalize(tag)
		if tag != "" {
			set[tag] = struct{}{}
		}
	}
	for _, rule := range t.rules {
		for _, keyword := range rule.Keywords {
			if strings.Contains(text, strings.ToLower(keyword)) {
				set[rule.Tag] = struct{}{}
				break
			}
		}
	}
	out := make([]string, 0, len(set))
	for tag := range set {
		out = append(out, tag)
	}
	sort.Strings(out)
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

func normalize(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	tag = strings.ReplaceAll(tag, "_", "-")
	tag = strings.Join(strings.Fields(tag), "-")
	return strings.Trim(tag, "-/#")
}
