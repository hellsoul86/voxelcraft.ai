package rules

import (
	"fmt"
	"strconv"
	"strings"
)

// jsonNumber is a tiny adapter so callers can pass json.Number without
// importing encoding/json in this package.
type jsonNumber interface {
	Float64() (float64, error)
}

func ParamFloat(params map[string]interface{}, key string) (float64, error) {
	v, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case jsonNumber:
		return x.Float64()
	default:
		return 0, fmt.Errorf("%s must be number", key)
	}
}

func ParamInt(params map[string]interface{}, key string) (int, error) {
	f, err := ParamFloat(params, key)
	if err != nil {
		return 0, err
	}
	return int(f), nil
}

func ParamString(params map[string]interface{}, key string) (string, error) {
	v, ok := params[key]
	if !ok {
		return "", fmt.Errorf("missing %s", key)
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("%s must be string", key)
	}
	return strings.TrimSpace(s), nil
}

func FloatToCanonString(f float64) string {
	// Stable representation suitable for hashing/digests.
	return strconv.FormatFloat(f, 'g', -1, 64)
}
