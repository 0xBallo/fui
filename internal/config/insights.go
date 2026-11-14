package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Default thresholds of the automatic insights; they mirror domain.DefaultInsightConfig.
const (
	DefaultInsightAnomalyFactor     = 2500
	DefaultInsightSubscriptionTol   = 50
	DefaultInsightSubscriptionCount = 3
	DefaultInsightTrendPermille     = 250
	DefaultInsightTrendMinMinor     = 5000
)

// InsightsConfig holds the insights.* thresholds. Ratios are permille (2500 = 2.5×) and
// TrendMinMinor is in minor units of the base currency.
type InsightsConfig struct {
	AnomalyFactorPermille int   `toml:"anomaly_factor_permille"`
	SubscriptionTolPermil int   `toml:"subscription_tolerance_permille"`
	SubscriptionMinCount  int   `toml:"subscription_min_occurrences"`
	TrendPermille         int   `toml:"trend_permille"`
	TrendMinMinor         int64 `toml:"trend_min_amount"`
}

func defaultInsights() InsightsConfig {
	return InsightsConfig{
		AnomalyFactorPermille: DefaultInsightAnomalyFactor,
		SubscriptionTolPermil: DefaultInsightSubscriptionTol,
		SubscriptionMinCount:  DefaultInsightSubscriptionCount,
		TrendPermille:         DefaultInsightTrendPermille,
		TrendMinMinor:         DefaultInsightTrendMinMinor,
	}
}

// insightField returns the value pointer and minimum allowed value of an insights.* key.
func (c *Config) insightField(key string) (p *int64, minimum int64, ok bool) {
	var n *int
	switch key {
	case "insights.anomaly_factor_permille":
		n, minimum = &c.Insights.AnomalyFactorPermille, 1000
	case "insights.subscription_tolerance_permille":
		n, minimum = &c.Insights.SubscriptionTolPermil, 0
	case "insights.subscription_min_occurrences":
		n, minimum = &c.Insights.SubscriptionMinCount, 2
	case "insights.trend_permille":
		n, minimum = &c.Insights.TrendPermille, 1
	case "insights.trend_min_amount":
		return &c.Insights.TrendMinMinor, 0, true
	default:
		return nil, 0, false
	}
	v := int64(*n)
	return &v, minimum, true
}

func (c *Config) getInsight(key string) (string, bool) {
	p, _, ok := c.insightField(key)
	if !ok {
		return "", false
	}
	return strconv.FormatInt(*p, 10), true
}

func (c *Config) setInsight(key, value string) (bool, error) {
	if !strings.HasPrefix(key, "insights.") {
		return false, nil
	}
	_, minimum, ok := c.insightField(key)
	if !ok {
		return false, nil
	}
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return true, fmt.Errorf("invalid value for key %q: %q (expected integer)", key, value)
	}
	if v < minimum {
		return true, fmt.Errorf("invalid value for key %q: %d (must be at least %d)", key, v, minimum)
	}
	switch key {
	case "insights.anomaly_factor_permille":
		c.Insights.AnomalyFactorPermille = int(v)
	case "insights.subscription_tolerance_permille":
		c.Insights.SubscriptionTolPermil = int(v)
	case "insights.subscription_min_occurrences":
		c.Insights.SubscriptionMinCount = int(v)
	case "insights.trend_permille":
		c.Insights.TrendPermille = int(v)
	default:
		c.Insights.TrendMinMinor = v
	}
	return true, nil
}

var insightKeys = []string{
	"insights.anomaly_factor_permille",
	"insights.subscription_tolerance_permille",
	"insights.subscription_min_occurrences",
	"insights.trend_permille",
	"insights.trend_min_amount",
}

func validateInsights(c *Config) error {
	for _, key := range insightKeys {
		p, minimum, _ := c.insightField(key)
		if *p < minimum {
			return fmt.Errorf("invalid value for key %q: %d (must be at least %d)", key, *p, minimum)
		}
	}
	return nil
}
