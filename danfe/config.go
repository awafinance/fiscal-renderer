package danfe

type TaxConfiguration string

const (
	TaxConfigurationStandardICMSIPI TaxConfiguration = "Standard ICMS and IPI"
	TaxConfigurationICMSST          TaxConfiguration = "ICMS ST only"
	TaxConfigurationWithoutIPI      TaxConfiguration = "Without IPI fields"
)

type InvoiceDisplay string

const (
	InvoiceDisplayDuplicatesOnly InvoiceDisplay = "Duplicatas Only"
	InvoiceDisplayFullDetails    InvoiceDisplay = "Full Details"
)

type FontType string

const (
	FontTypeCourier FontType = "Courier"
	FontTypeTimes   FontType = "Times"
)

type FontSize float64

const (
	FontSizeSmall FontSize = 1.0
	FontSizeBig   FontSize = 1.35
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

type ReceiptPosition string

const (
	ReceiptPositionTop    ReceiptPosition = "top"
	ReceiptPositionBottom ReceiptPosition = "bottom"
	ReceiptPositionLeft   ReceiptPosition = "left"
)

type ProductDescriptionConfig struct {
	DisplayBranch         bool
	DisplayANP            bool
	DisplayANVISA         bool
	BranchInfoPrefix      string
	DisplayAdditionalInfo bool
}

type FooterStamp struct {
	Logo         string
	LogoBytes    []byte
	Text         string
	Height       float64
	LogoMaxWidth float64
	Spacing      float64
}

type Config struct {
	Logo                     string
	LogoBytes                []byte
	Margins                  Margins
	ReceiptPosition          ReceiptPosition
	DecimalConfig            DecimalConfig
	TaxConfiguration         TaxConfiguration
	InvoiceDisplay           InvoiceDisplay
	FontType                 FontType
	FontSize                 FontSize
	DisplayPISCOFINS         bool
	WatermarkCancelled       bool
	InfCplSemicolonNewline   bool
	ProductDescriptionConfig ProductDescriptionConfig
	FooterStamp              FooterStamp
}

func DefaultMargins() Margins {
	return Margins{Top: 5, Right: 5, Bottom: 5, Left: 5}
}

func DefaultDecimalConfig() DecimalConfig {
	return DecimalConfig{PricePrecision: 4, QuantityPrecision: 4}
}

func DefaultProductDescriptionConfig() ProductDescriptionConfig {
	return ProductDescriptionConfig{DisplayAdditionalInfo: true}
}

func DefaultFooterStamp() FooterStamp {
	return FooterStamp{Height: 5, LogoMaxWidth: 20, Spacing: 1}
}

func DefaultConfig() Config {
	return Config{
		Margins:                  DefaultMargins(),
		ReceiptPosition:          ReceiptPositionTop,
		DecimalConfig:            DefaultDecimalConfig(),
		TaxConfiguration:         TaxConfigurationStandardICMSIPI,
		InvoiceDisplay:           InvoiceDisplayFullDetails,
		FontType:                 FontTypeTimes,
		FontSize:                 FontSizeSmall,
		ProductDescriptionConfig: DefaultProductDescriptionConfig(),
		FooterStamp:              DefaultFooterStamp(),
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
	if normalized.ReceiptPosition == "" {
		normalized.ReceiptPosition = defaults.ReceiptPosition
	}
	if normalized.TaxConfiguration == "" {
		normalized.TaxConfiguration = defaults.TaxConfiguration
	}
	if normalized.InvoiceDisplay == "" {
		normalized.InvoiceDisplay = defaults.InvoiceDisplay
	}
	if normalized.FontType == "" {
		normalized.FontType = defaults.FontType
	}
	if normalized.FontSize == 0 {
		normalized.FontSize = defaults.FontSize
	}
	if normalized.ProductDescriptionConfig == (ProductDescriptionConfig{}) {
		normalized.ProductDescriptionConfig = defaults.ProductDescriptionConfig
	}
	if footerStampIsZero(normalized.FooterStamp) {
		normalized.FooterStamp = defaults.FooterStamp
	} else {
		if normalized.FooterStamp.Height == 0 {
			normalized.FooterStamp.Height = defaults.FooterStamp.Height
		}
		if normalized.FooterStamp.LogoMaxWidth == 0 {
			normalized.FooterStamp.LogoMaxWidth = defaults.FooterStamp.LogoMaxWidth
		}
		if normalized.FooterStamp.Spacing == 0 {
			normalized.FooterStamp.Spacing = defaults.FooterStamp.Spacing
		}
	}
	return normalized
}

func footerStampIsZero(stamp FooterStamp) bool {
	return stamp.Logo == "" &&
		len(stamp.LogoBytes) == 0 &&
		stamp.Text == "" &&
		stamp.Height == 0 &&
		stamp.LogoMaxWidth == 0 &&
		stamp.Spacing == 0
}
