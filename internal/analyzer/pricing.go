package analyzer

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

// ModelPricing defines token cost per 1,000,000 tokens in USD.
type ModelPricing struct {
	InputPerMillion     float64 `json:"input_per_million"`
	CacheReadPerMillion float64 `json:"cache_read_per_million"`
	OutputPerMillion    float64 `json:"output_per_million"`
}

const (
	DefaultUSDToVNDRate = 25400.0
)

// DefaultPricingMap holds known pricing per model tier.
var DefaultPricingMap = map[string]ModelPricing{
	"gemini-flash": {
		InputPerMillion:     0.10,
		CacheReadPerMillion: 0.025,
		OutputPerMillion:    0.40,
	},
	"gemini-pro": {
		InputPerMillion:     1.25,
		CacheReadPerMillion: 0.3125,
		OutputPerMillion:    5.00,
	},
	"claude": {
		InputPerMillion:     3.00,
		CacheReadPerMillion: 0.30,
		OutputPerMillion:    15.00,
	},
}

var (
	pricingLock sync.RWMutex
	customMap   = make(map[string]ModelPricing)
)

// LoadPricingOverride loads pricing configuration from a JSON file or predefined model tier.
func LoadPricingOverride(spec string) error {
	if spec == "" {
		return nil
	}

	pricingLock.Lock()
	defer pricingLock.Unlock()

	// Check if spec is a valid JSON file
	if data, err := os.ReadFile(spec); err == nil {
		var parsed map[string]ModelPricing
		if jsonErr := json.Unmarshal(data, &parsed); jsonErr == nil {
			for k, v := range parsed {
				customMap[strings.ToLower(k)] = v
			}
			return nil
		}
	}

	// Or if it's a model shorthand
	lower := strings.ToLower(spec)
	if p, ok := DefaultPricingMap[lower]; ok {
		customMap["default"] = p
	}
	return nil
}

// ResolvePricing returns the appropriate ModelPricing based on model name.
func ResolvePricing(modelName string) ModelPricing {
	pricingLock.RLock()
	defer pricingLock.RUnlock()

	if def, ok := customMap["default"]; ok {
		return def
	}

	lower := strings.ToLower(modelName)
	for k, v := range customMap {
		if strings.Contains(lower, k) {
			return v
		}
	}

	if strings.Contains(lower, "pro") {
		return DefaultPricingMap["gemini-pro"]
	}
	if strings.Contains(lower, "claude") {
		return DefaultPricingMap["claude"]
	}
	// Default to gemini-flash
	return DefaultPricingMap["gemini-flash"]
}

// CalculateCost computes total cost in USD and VND for the given token amounts.
func CalculateCost(modelName string, inputTokens, cacheReadTokens, outputTokens int64) (costUSD float64, costVND float64) {
	pricing := ResolvePricing(modelName)

	costUSD = (float64(inputTokens) / 1_000_000.0 * pricing.InputPerMillion) +
		(float64(cacheReadTokens) / 1_000_000.0 * pricing.CacheReadPerMillion) +
		(float64(outputTokens) / 1_000_000.0 * pricing.OutputPerMillion)

	costVND = costUSD * DefaultUSDToVNDRate
	return costUSD, costVND
}
