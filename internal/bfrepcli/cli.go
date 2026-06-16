package bfrepcli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/awafinance/fiscal-renderer/dacce"
	"github.com/awafinance/fiscal-renderer/dacte"
	"github.com/awafinance/fiscal-renderer/damdfe"
	"github.com/awafinance/fiscal-renderer/danfe"
	"github.com/awafinance/fiscal-renderer/danfse"
	"github.com/awafinance/fiscal-renderer/internal/images"
	"gopkg.in/yaml.v3"
)

const Version = "0.8.0"

type configFile struct {
	Issuer       issuerConfig `yaml:"ISSUER"`
	Logo         string       `yaml:"LOGO"`
	TopMargin    *float64     `yaml:"TOP_MARGIN"`
	RightMargin  *float64     `yaml:"RIGHT_MARGIN"`
	BottomMargin *float64     `yaml:"BOTTOM_MARGIN"`
	LeftMargin   *float64     `yaml:"LEFT_MARGIN"`
}

type issuerConfig struct {
	Nome   string `yaml:"nome"`
	End    string `yaml:"end"`
	Bairro string `yaml:"bairro"`
	CEP    string `yaml:"cep"`
	Cidade string `yaml:"cidade"`
	UF     string `yaml:"uf"`
	Fone   string `yaml:"fone"`
}

func Run(args []string, stdout, stderr io.Writer, cwd string) int {
	if len(args) == 0 {
		rootHelp(stdout)
		return 0
	}
	if args[0] == "--version" || args[0] == "-v" {
		fmt.Fprintf(stdout, "bfrep version %s\n", Version)
		return 0
	}
	if args[0] == "--help" {
		rootHelp(stdout)
		return 0
	}
	command := args[0]
	if !isCommand(command) {
		rootUsageError(stderr, fmt.Sprintf("No such command '%s'.", command))
		return 2
	}
	if len(args) == 2 && args[1] == "--help" {
		commandHelp(stdout, command)
		return 0
	}
	if len(args) == 1 {
		commandUsageError(stderr, command, "Missing argument 'XML'.")
		return 2
	}
	if len(args) > 2 {
		commandUsageError(stderr, command, fmt.Sprintf("Got unexpected extra argument (%s)", args[2]))
		return 2
	}

	xmlArg := args[1]
	xmlPath := xmlArg
	if !filepath.IsAbs(xmlPath) {
		xmlPath = filepath.Join(cwd, xmlPath)
	}
	if _, err := os.Stat(xmlPath); os.IsNotExist(err) {
		commandUsageError(stderr, command, fmt.Sprintf("Invalid value for 'XML': Path '%s' does not exist.", xmlArg))
		return 2
	}
	xmlContent, err := os.ReadFile(xmlPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading XML file: %v\n", err)
		return 1
	}
	outputPath := filepath.Join(cwd, trimExt(filepath.Base(xmlPath))+".pdf")

	config, err := loadConfig(cwd, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "Error loading config.yaml: %v\n", err)
		return 1
	}

	switch command {
	case "dacce":
		issuer := config.issuer()
		err = dacce.RenderFile(string(xmlContent), outputPath, &dacce.Config{Issuer: issuer})
		if err == nil {
			fmt.Fprintf(stdout, "DACCe generated successfully: %s\n", outputPath)
		}
	case "danfe":
		cfg, ok := config.danfeConfig(cwd, stdout)
		err = danfe.RenderFile(string(xmlContent), outputPath, cfg)
		if err == nil && ok {
			fmt.Fprintf(stdout, "DANFE generated successfully: %s\n", outputPath)
		}
	case "dacte":
		cfg, ok := config.dacteConfig(cwd, stdout)
		err = dacte.RenderFile(string(xmlContent), outputPath, cfg)
		if err == nil && ok {
			fmt.Fprintf(stdout, "DACTE generated successfully: %s\n", outputPath)
		}
	case "damdfe":
		cfg, ok := config.damdfeConfig(cwd, stdout)
		err = damdfe.RenderFile(string(xmlContent), outputPath, cfg)
		if err == nil && ok {
			fmt.Fprintf(stdout, "DAMDFE generated successfully: %s\n", outputPath)
		}
	case "danfse":
		cfg := config.danfseConfig()
		err = danfse.RenderFile(string(xmlContent), outputPath, cfg)
		if err == nil {
			fmt.Fprintf(stdout, "DANFSE generated successfully: %s\n", outputPath)
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "Error generating PDF: %v\n", err)
		return 1
	}
	return 0
}

func rootUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: bfrep [OPTIONS] COMMAND [ARGS]...")
}

func rootUsageError(w io.Writer, message string) {
	rootUsage(w)
	fmt.Fprintln(w, "Try 'bfrep --help' for help.")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Error: %s\n", message)
}

func rootHelp(w io.Writer) {
	rootUsage(w)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -v, --version  Show the version and exit.")
	fmt.Fprintln(w, "  --help         Show this message and exit.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  dacce")
	fmt.Fprintln(w, "  dacte")
	fmt.Fprintln(w, "  damdfe")
	fmt.Fprintln(w, "  danfe")
	fmt.Fprintln(w, "  danfse")
}

func commandUsage(w io.Writer, command string) {
	fmt.Fprintf(w, "Usage: bfrep %s [OPTIONS] XML\n", command)
}

func commandUsageError(w io.Writer, command string, message string) {
	commandUsage(w, command)
	fmt.Fprintf(w, "Try 'bfrep %s --help' for help.\n", command)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Error: %s\n", message)
}

func commandHelp(w io.Writer, command string) {
	commandUsage(w, command)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --help  Show this message and exit.")
}

func isCommand(command string) bool {
	switch command {
	case "danfe", "dacce", "dacte", "damdfe", "danfse":
		return true
	default:
		return false
	}
}

func loadConfig(cwd string, stdout io.Writer) (configFile, error) {
	path := filepath.Join(cwd, "config.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		fmt.Fprintln(stdout, "Config file 'config.yaml' not found. Using default configuration.")
		return configFile{}, nil
	}
	if err != nil {
		return configFile{}, err
	}
	var config configFile
	if err := yaml.Unmarshal(data, &config); err != nil {
		return configFile{}, err
	}
	return config, nil
}

func (c configFile) issuer() dacce.Issuer {
	if c.Issuer == (issuerConfig{}) {
		return dacce.DefaultIssuer()
	}
	return dacce.Issuer{
		Name:         c.Issuer.Nome,
		Address:      c.Issuer.End,
		Neighborhood: c.Issuer.Bairro,
		CEP:          c.Issuer.CEP,
		City:         c.Issuer.Cidade,
		UF:           c.Issuer.UF,
		Phone:        c.Issuer.Fone,
	}
}

func (c configFile) danfeConfig(cwd string, stdout io.Writer) (*danfe.Config, bool) {
	cfg := danfe.DefaultConfig()
	cfg.Margins = danfe.Margins(c.margins())
	cfg.Logo = c.logo(cwd, stdout)
	return &cfg, true
}

func (c configFile) dacteConfig(cwd string, stdout io.Writer) (*dacte.Config, bool) {
	cfg := dacte.DefaultConfig()
	cfg.Margins = dacte.Margins(c.margins())
	cfg.Logo = c.logo(cwd, stdout)
	return &cfg, true
}

func (c configFile) damdfeConfig(cwd string, stdout io.Writer) (*damdfe.Config, bool) {
	cfg := damdfe.DefaultConfig()
	cfg.Margins = damdfe.Margins(c.margins())
	cfg.Logo = c.logo(cwd, stdout)
	return &cfg, true
}

func (c configFile) danfseConfig() *danfse.Config {
	cfg := danfse.DefaultConfig()
	cfg.Margins = danfse.Margins(c.margins())
	return &cfg
}

func (c configFile) margins() struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
} {
	m := struct {
		Top    float64
		Right  float64
		Bottom float64
		Left   float64
	}{Top: 5, Right: 5, Bottom: 5, Left: 5}
	if c.TopMargin != nil {
		m.Top = *c.TopMargin
	}
	if c.RightMargin != nil {
		m.Right = *c.RightMargin
	}
	if c.BottomMargin != nil {
		m.Bottom = *c.BottomMargin
	}
	if c.LeftMargin != nil {
		m.Left = *c.LeftMargin
	}
	return m
}

func (c configFile) logo(cwd string, stdout io.Writer) string {
	path, exists, err := images.ResolveOptionalPath(cwd, c.Logo)
	if err != nil || c.Logo == "" {
		return ""
	}
	if !exists {
		fmt.Fprintln(stdout, "Logo file not found, proceeding without logo.")
		return ""
	}
	return path
}

func trimExt(path string) string {
	ext := filepath.Ext(path)
	return path[:len(path)-len(ext)]
}
