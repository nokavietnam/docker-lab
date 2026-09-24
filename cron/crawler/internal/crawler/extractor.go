package crawler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"muaban-crawler/internal/model"
)

var (
	ErrNextDataNotFound = errors.New("__NEXT_DATA__ tag not found in response HTML")
	ErrInvalidScriptEnd = errors.New("closing </script> tag not found")
)

// ExtractListings extracts the listings and total count from Muaban.net's Next.js SSR HTML
// Using direct byte indexing avoids creating a heavy HTML DOM tree, keeping RAM/CPU minimal.
func ExtractListings(htmlBytes []byte) ([]model.Property, int, error) {
	tagIdx := bytes.Index(htmlBytes, []byte("__NEXT_DATA__"))
	if tagIdx == -1 {
		return nil, 0, ErrNextDataNotFound
	}

	startRel := bytes.IndexByte(htmlBytes[tagIdx:], '>')
	if startRel == -1 {
		return nil, 0, ErrNextDataNotFound
	}
	jsonStart := tagIdx + startRel + 1

	endRel := bytes.Index(htmlBytes[jsonStart:], []byte("</script>"))
	if endRel == -1 {
		return nil, 0, ErrInvalidScriptEnd
	}
	jsonEnd := jsonStart + endRel

	rawJSON := htmlBytes[jsonStart:jsonEnd]

	var wrapper model.NextDataWrapper
	if err := json.Unmarshal(rawJSON, &wrapper); err != nil {
		return nil, 0, fmt.Errorf("failed to decode __NEXT_DATA__ json: %w", err)
	}

	classified := wrapper.Props.PageProps.Classified
	totalItems := classified.Total
	items := make([]model.Property, 0, len(classified.Items))

	for _, rawItem := range classified.Items {
		var prop model.Property
		if err := json.Unmarshal(rawItem, &prop); err != nil {
			continue // skip malformed item gracefully
		}
		prop.RawData = rawItem
		items = append(items, prop)
	}

	return items, totalItems, nil
}
