package pkgsiteapi

import "encoding/json"

// UnmarshalJSON accepts the object-shaped paginated items declared by the API
// schema plus string items returned by some endpoints, such as imported-by for
// standard-library packages.
func (p *PaginatedResponse) UnmarshalJSON(data []byte) error {
	type paginatedResponse PaginatedResponse
	var raw struct {
		Items *[]json.RawMessage `json:"items,omitempty"`
		*paginatedResponse
	}
	raw.paginatedResponse = (*paginatedResponse)(p)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Items == nil {
		p.Items = nil
		return nil
	}
	items := make([]map[string]any, 0, len(*raw.Items))
	for _, item := range *raw.Items {
		normalized, err := paginatedItem(item)
		if err != nil {
			return err
		}
		items = append(items, normalized)
	}
	p.Items = &items
	return nil
}

func paginatedItem(data json.RawMessage) (map[string]any, error) {
	var object map[string]any
	if err := json.Unmarshal(data, &object); err == nil {
		return object, nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		return map[string]any{"path": text}, nil
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return map[string]any{"value": value}, nil
}
