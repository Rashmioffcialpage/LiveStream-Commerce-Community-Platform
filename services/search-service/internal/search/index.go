// Package search wraps OpenSearch: one index ("channels"), one document
// per channel, kept in sync by Kafka consumers in the handler package
// rather than search-service owning any writes of its own -- this
// service only ever reacts to channel-events/stream-events, it's not a
// system of record for anything.
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

const IndexName = "channels"

type Client struct {
	api *opensearchapi.Client
}

func New(url string) (*Client, error) {
	api, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{Addresses: []string{url}},
	})
	if err != nil {
		return nil, err
	}
	return &Client{api: api}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.api.Info(ctx, nil)
	return err
}

type ChannelDoc struct {
	ID           string   `json:"id"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Description  string   `json:"description"`
	CreatorID    string   `json:"creator_id"`
	CreatorName  string   `json:"creator_name"`
	StreamTitles []string `json:"stream_titles"`
	Tags         []string `json:"tags"`
}

// EnsureIndex creates the index with an explicit mapping on first boot.
// The "already exists" case (every boot after the first) isn't an
// error -- it's the expected steady state.
func (c *Client) EnsureIndex(ctx context.Context) error {
	mapping := `{
		"mappings": {
			"properties": {
				"name": {"type": "text"},
				"category": {"type": "keyword"},
				"description": {"type": "text"},
				"creator_name": {"type": "text"},
				"stream_titles": {"type": "text"},
				"tags": {"type": "keyword"}
			}
		}
	}`
	_, err := c.api.Indices.Create(ctx, opensearchapi.IndicesCreateReq{
		Index: IndexName,
		Body:  strings.NewReader(mapping),
	})
	if err != nil && !strings.Contains(err.Error(), "resource_already_exists_exception") {
		return err
	}
	return nil
}

// IndexChannel creates or fully replaces a channel's document -- used on
// channel-created, where there's no prior document to merge into.
func (c *Client) IndexChannel(ctx context.Context, doc ChannelDoc) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	_, err = c.api.Index(ctx, opensearchapi.IndexReq{
		Index:      IndexName,
		DocumentID: doc.ID,
		Body:       bytes.NewReader(body),
	})
	return err
}

// AppendStreamTitle partially updates an existing channel document via a
// Painless script rather than a full IndexChannel replace, so a
// stream-created event doesn't need to know or refetch the rest of the
// channel's fields just to add one title. Keeps the last 10 titles --
// enough to search recent stream history without the array growing
// unbounded for a long-lived channel.
func (c *Client) AppendStreamTitle(ctx context.Context, channelID, title string, tags []string) error {
	update := map[string]any{
		"script": map[string]any{
			"source": `
				if (ctx._source.stream_titles == null) { ctx._source.stream_titles = []; }
				ctx._source.stream_titles.add(params.title);
				if (ctx._source.stream_titles.size() > 10) { ctx._source.stream_titles.remove(0); }
				if (ctx._source.tags == null) { ctx._source.tags = []; }
				for (def t : params.tags) {
					if (!ctx._source.tags.contains(t)) { ctx._source.tags.add(t); }
				}
			`,
			"params": map[string]any{"title": title, "tags": tags},
		},
	}
	body, err := json.Marshal(update)
	if err != nil {
		return err
	}
	_, err = c.api.Update(ctx, opensearchapi.UpdateReq{
		Index:      IndexName,
		DocumentID: channelID,
		Body:       bytes.NewReader(body),
	})
	return err
}

// Search runs a multi_match across everything the spec asks for --
// creator, category, stream title, tags -- weighted so a channel-name
// match ranks above an incidental tag match.
func (c *Client) Search(ctx context.Context, query string) ([]ChannelDoc, error) {
	q := map[string]any{
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":  query,
				"fields": []string{"name^3", "creator_name^2", "stream_titles^2", "category", "tags", "description"},
			},
		},
		"size": 20,
	}
	body, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	resp, err := c.api.Search(ctx, &opensearchapi.SearchReq{
		Indices: []string{IndexName},
		Body:    bytes.NewReader(body),
	})
	if err != nil {
		return nil, err
	}

	docs := make([]ChannelDoc, 0, len(resp.Hits.Hits))
	for _, hit := range resp.Hits.Hits {
		var doc ChannelDoc
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			continue
		}
		docs = append(docs, doc)
	}
	return docs, nil
}
