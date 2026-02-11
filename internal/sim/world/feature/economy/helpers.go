package economy

import (
	"fmt"
	"sort"

	"voxelcraft.ai/internal/protocol"
)

func ParseItemPairs(pairs [][]interface{}) (map[string]int, error) {
	out := map[string]int{}
	for _, p := range pairs {
		if len(p) != 2 {
			return nil, fmt.Errorf("pair must have len=2")
		}
		item, ok := p[0].(string)
		if !ok || item == "" {
			return nil, fmt.Errorf("item id must be string")
		}
		n := 0
		switch v := p[1].(type) {
		case float64:
			n = int(v)
		case int:
			n = v
		case int64:
			n = int(v)
		default:
			return nil, fmt.Errorf("count must be number")
		}
		if n <= 0 {
			return nil, fmt.Errorf("count must be > 0")
		}
		out[item] += n
	}
	return out, nil
}

func EncodeItemPairs(m map[string]int) [][]interface{} {
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v > 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([][]interface{}, 0, len(keys))
	for _, k := range keys {
		out = append(out, []interface{}{k, m[k]})
	}
	return out
}

func HasItems(inv map[string]int, want map[string]int) bool {
	if len(want) == 0 {
		return true
	}
	for item, c := range want {
		if inv[item] < c {
			return false
		}
	}
	return true
}

func ApplyTransfer(src, dst map[string]int, items map[string]int) {
	for item, c := range items {
		src[item] -= c
		dst[item] += c
	}
}

func ApplyTransferWithTax(src, dst map[string]int, items map[string]int, taxSink map[string]int, taxRate float64) {
	if taxRate <= 0 {
		ApplyTransfer(src, dst, items)
		return
	}
	if taxRate > 1 {
		taxRate = 1
	}
	for item, c := range items {
		src[item] -= c
		tax := int(float64(c) * taxRate) // floor
		if tax < 0 {
			tax = 0
		}
		if tax > c {
			tax = c
		}
		dst[item] += c - tax
		if taxSink != nil && tax > 0 {
			taxSink[item] += tax
		}
	}
}

func CalcTax(items map[string]int, taxRate float64) map[string]int {
	if taxRate <= 0 || len(items) == 0 {
		return nil
	}
	if taxRate > 1 {
		taxRate = 1
	}
	out := map[string]int{}
	for item, c := range items {
		if c <= 0 {
			continue
		}
		tax := int(float64(c) * taxRate) // floor
		if tax <= 0 {
			continue
		}
		if tax > c {
			tax = c
		}
		out[item] = tax
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func StacksToMap(stacks []protocol.ItemStack) map[string]int {
	out := map[string]int{}
	for _, s := range stacks {
		if s.Item == "" || s.Count <= 0 {
			continue
		}
		out[s.Item] += s.Count
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
