package danfse

import "github.com/awafinance/fiscal-renderer/internal/footer"

// FooterStamp is the optional marketing/footer note drawn at the bottom of each
// page. Its Text field supports markdown-ish formatting (**bold**, *italic*,
// [label](url)). The zero value draws nothing.
type FooterStamp = footer.Stamp

type FontType string

const (
	FontTypeCourier FontType = "Courier"
	FontTypeTimes   FontType = "Times"
)

type Margins struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

type DecimalConfig struct {
	PricePrecision    int
	QuantityPrecision int
}

type Config struct {
	Margins            Margins
	DecimalConfig      DecimalConfig
	FontType           FontType
	WatermarkCancelled bool
	FooterStamp        FooterStamp
}

func bottomMargin(config Config) float64 {
	return config.Margins.Bottom + config.FooterStamp.Reserve()
}

func DefaultMargins() Margins {
	return Margins{Top: 5, Right: 5, Bottom: 5, Left: 5}
}

func DefaultDecimalConfig() DecimalConfig {
	return DecimalConfig{PricePrecision: 4, QuantityPrecision: 4}
}

func DefaultConfig() Config {
	return Config{
		Margins:       DefaultMargins(),
		DecimalConfig: DefaultDecimalConfig(),
		FontType:      FontTypeTimes,
	}
}

func normalizeConfig(config *Config) Config {
	if config == nil {
		return DefaultConfig()
	}
	normalized := *config
	defaults := DefaultConfig()
	if normalized.Margins == (Margins{}) {
		normalized.Margins = defaults.Margins
	}
	if normalized.DecimalConfig == (DecimalConfig{}) {
		normalized.DecimalConfig = defaults.DecimalConfig
	}
	if normalized.FontType == "" {
		normalized.FontType = defaults.FontType
	}
	normalized.FooterStamp = normalized.FooterStamp.Normalize(footer.Default())
	return normalized
}
