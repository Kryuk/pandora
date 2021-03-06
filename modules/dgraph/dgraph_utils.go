package dgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/dgo"
	"github.com/dgraph-io/dgo/protos/api"
	"github.com/sergeyt/pandora/modules/apiutil"
	"github.com/sergeyt/pandora/modules/utils"
)

func NodeLabel(resourceType string) string {
	return "_" + resourceType
}

func ReadList(ctx context.Context, tx *dgo.Txn, label string, pg apiutil.Pagination) ([]map[string]interface{}, error) {
	query := fmt.Sprintf(`{
  items(func: has(%s), offset: %d, first: %d) {
    uid
    expand(_all_)
  }
  total(func: has(%s)) {
    count: count(uid)
  }
}`, label, pg.Offset, pg.Limit, label)

	resp, err := tx.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []map[string]interface{} `json:"items"`
	}
	err = json.Unmarshal(resp.GetJson(), &result)

	if err != nil {
		return nil, err
	}

	return result.Results, nil
}

func ReadNode(ctx context.Context, tx *dgo.Txn, id string) (map[string]interface{}, error) {
	query := fmt.Sprintf(`{
  node(func: uid(%s)) {
    expand(_all_) {
      expand(_all_)
    }
  }
}`, id)

	resp, err := tx.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []map[string]interface{} `json:"node"`
	}
	err = json.Unmarshal(resp.GetJson(), &result)

	if err != nil {
		return nil, err
	}

	if len(result.Results) == 0 {
		return nil, fmt.Errorf("not found")
	}

	d := result.Results[0]
	d["uid"] = id

	return d, nil
}

type Mutation struct {
	Input     utils.OrderedJSON
	NodeLabel string
	ID        string
	By        string
}

func Mutate(ctx context.Context, tx *dgo.Txn, m Mutation) ([]map[string]interface{}, error) {
	id := m.ID
	isNew := len(id) == 0
	now := time.Now()

	in := m.Input
	in["modified_at"] = now
	in["modified_by"] = m.By

	if isNew {
		in[m.NodeLabel] = ""
		in["created_at"] = now
		in["created_by"] = m.By
	} else {
		in["uid"] = id
	}

	data, err := in.ToJSON("uid", m.NodeLabel)
	if err != nil {
		return nil, err
	}

	resp, err := tx.Mutate(ctx, &api.Mutation{
		SetJson: data,
	})
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	if isNew {
		results = make([]map[string]interface{}, len(resp.Uids))
		i := 0
		for _, uid := range resp.Uids {
			result, err := ReadNode(ctx, tx, uid)
			if err != nil {
				return nil, err
			}
			results[i] = result
			i = i + 1
			if len(results) == 1 {
				id = uid
			}
		}
	} else {
		result, err := ReadNode(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		results = []map[string]interface{}{result}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return results, nil
}
