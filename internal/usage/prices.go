package usage

import "strings"

// price is in USD per million tokens.
type price struct {
	input, output float64
}

// shortcut: the prices are fixed in code, update them when Anthropic changes its prices.
// The longest matching prefix wins. Cache write is 1.25 × input, cache read is 0.1 × input.
var prices = map[string]price{
	"claude-opus-":       {5, 25},
	"claude-opus-4-0":    {15, 75},
	"claude-opus-4-1":    {15, 75},
	"claude-opus-4-2025": {15, 75},
	"claude-sonnet-":     {3, 15},
	"claude-haiku-":      {1, 5},
	"claude-3-5-haiku":   {0.8, 4},
}

func priceFor(model string) (price, bool) {
	var best string
	for prefix := range prices {
		if strings.HasPrefix(model, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	p, ok := prices[best]
	return p, ok
}
