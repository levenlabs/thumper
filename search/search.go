// Package search deals with all search queries going to elasticsearch, and
// returning their result
package search

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/levenlabs/thumper/config"
)

// Hit describes one of the documents matched by a search
type Hit struct {
	Index  string                 `json:"_index"`  // The index the hit came from
	Type   string                 `json:"_type"`   // The type the document is
	ID     string                 `json:"_id"`     // The unique id of the document
	Score  float64                `json:"_score"`  // The document's score relative to the search
	Source map[string]interface{} `json:"_source"` // The actual document
}

// HitInfo describes information in the Result related to the actual hits
type HitInfo struct {
	HitCount    uint64  `json:"total"`     // The total number of documents matched
	HitMaxScore float64 `json:"max_score"` // The maximum score of all the documents matched
	Hits        []Hit   `json:"hits"`      // The actual documents matched
}

// Result describes the returned data from a search search
type Result struct {
	TookMS       uint64                          `json:"took"`      // Time search took to complete, in milliseconds
	TimedOut     bool                            `json:"timed_out"` // Whether or not the search timed out
	HitInfo      `json:"hits" luautil:",inline"` // Information related to the actual hits
	Aggregations map[string]interface{}          `json:"aggregations"` // Information related to aggregations in the query
}

type elasticError struct {
	Error string `json:"error"`
}

// Search performs a search against the given elasticsearch index for
// documents of the given type. The search must json marshal into a valid
// elasticsearch request body query
// (see https://www.elastic.co/guide/en/elasticsearch/reference/current/search-request-body.html)
func Search(index, typ string, search interface{}) (Result, error) {
	u := fmt.Sprintf("http://%s/%s/%s/_search", config.ElasticSearchAddr, index, typ)
	body, err := json.Marshal(search)
	if err != nil {
		return Result{}, err
	}

	req, err := http.NewRequest("GET", u, bytes.NewBuffer(body))
	if err != nil {
		return Result{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)

	if resp.StatusCode != 200 {
		var e elasticError
		if err := dec.Decode(&e); err != nil {
			return Result{}, err
		}
		return Result{}, errors.New(e.Error)
	}

	var result Result
	if err := dec.Decode(&result); err != nil {
		return result, err
	} else if result.TimedOut {
		return result, errors.New("search timed out in elasticsearch")
	}

	return result, nil
}
