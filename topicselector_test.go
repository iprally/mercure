package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchCache(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(DefaultTopicSelectorStoreCacheSize)
	require.NoError(t, err)

	assert.False(t, tss.match("foo", "bar"))

	assert.True(t, tss.match("https://example.com/foo/bar", "https://example.com/{foo}/bar"))

	// Template should be cached
	_, found := tss.templateCache.GetIfPresent("https://example.com/{foo}/bar")
	assert.True(t, found)

	// Match result should be cached (struct key, no collision possible)
	_, found = tss.matchCache.GetIfPresent(matchCacheKey{
		topicSelector: "https://example.com/{foo}/bar",
		topic:         "https://example.com/foo/bar",
	})
	assert.True(t, found)

	assert.True(t, tss.match("https://example.com/foo/bar", "https://example.com/{foo}/bar"))
	assert.False(t, tss.match("https://example.com/foo/bar/baz", "https://example.com/{foo}/bar"))

	assert.True(t, tss.match("https://example.com/kevin/dunglas", "https://example.com/{fistname}/{lastname}"))
	assert.True(t, tss.match("https://example.com/foo/bar", "*"))
	assert.True(t, tss.match("https://example.com/foo/bar", "https://example.com/foo/bar"))
	assert.True(t, tss.match("foo", "foo"))
	assert.False(t, tss.match("foo", "bar"))
}

func TestMatchCacheKeyNoCollision(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(DefaultTopicSelectorStoreCacheSize)
	require.NoError(t, err)

	// With struct keys, these are naturally distinct — no encoding tricks needed.
	// Use URI templates to ensure results get cached.
	assert.True(t, tss.match("https://example.com/a", "https://example.com/{x}"))
	assert.False(t, tss.match("https://other.com/a", "https://example.com/{x}"))

	// Verify independent cache entries exist
	_, found := tss.matchCache.GetIfPresent(matchCacheKey{
		topicSelector: "https://example.com/{x}",
		topic:         "https://example.com/a",
	})
	assert.True(t, found)

	_, found = tss.matchCache.GetIfPresent(matchCacheKey{
		topicSelector: "https://example.com/{x}",
		topic:         "https://other.com/a",
	})
	assert.True(t, found)
}
