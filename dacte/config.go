package dacte

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

type ModalType string

const (
	ModalTypeRodoviario  ModalType = "RODOVIÁRIO"
	ModalTypeAereo       ModalType = "AÉREO"
	ModalTypeAquaviario  ModalType = "AQUAVIÁRIO"
	ModalTypeFerroviario ModalType = "FERROVIÁRIO"
	ModalTypeDutoviario  ModalType = "DUTOVIÁRIO"
	ModalTypeMultimodal  ModalType = "MULTIMODAL"
)

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

type Config struct {
	Logo               string
	LogoBytes          []byte
	Margins            Margins
	ReceiptPosition    ReceiptPosition
	DecimalConfig      DecimalConfig
	FontType           FontType
	WatermarkCancelled bool
	DisplayIBSCBS      bool
}

func DefaultMargins() Margins {
	return Margins{Top: 5, Right: 5, Bottom: 5, Left: 5}
}

func DefaultDecimalConfig() DecimalConfig {
	return DecimalConfig{PricePrecision: 4, QuantityPrecision: 4}
}

func DefaultConfig() Config {
	return Config{
		Margins:         DefaultMargins(),
		ReceiptPosition: ReceiptPositionTop,
		DecimalConfig:   DefaultDecimalConfig(),
		FontType:        FontTypeTimes,
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
	if normalized.FontType == "" {
		normalized.FontType = defaults.FontType
	}
	return normalized
}
