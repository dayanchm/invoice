package invoice

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Seller           Party  `yaml:"seller"`
	Customer         Party  `yaml:"customer"`
	Currency         string `yaml:"currency"`
	PaymentTerms     string `yaml:"payment_terms"`
	PaymentTermsDays int    `yaml:"payment_terms_days"`
	TaxBasisPoints   int64  `yaml:"tax_basis_points"`
	Items            []Item `yaml:"items"`
	PaymentDetails   string `yaml:"payment_details"`
	Notes            string `yaml:"notes"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	c := Config{Currency: "CHF", PaymentTerms: "days", PaymentTermsDays: 30}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, fmt.Errorf("config must contain exactly one YAML document")
		}
		return Config{}, fmt.Errorf("decode trailing config data: %w", err)
	}
	// YAML v3 coerces floats to integers. Check scalar tags as well so a price
	// such as 10.5 cents or quantity 1.5 cannot silently become 10 or 1.
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return Config{}, fmt.Errorf("decode config structure: %w", err)
	}
	if err := checkIntegerFields(&document); err != nil {
		return Config{}, err
	}
	return c, nil
}

func checkIntegerFields(node *yaml.Node) error {
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			switch key.Value {
			case "quantity", "unit_price_cents", "payment_terms_days", "tax_basis_points":
				if value.Kind == yaml.AliasNode {
					value = value.Alias
				}
				if value.Tag != "!!int" {
					return fmt.Errorf("config line %d: %s must be an integer", key.Line, key.Value)
				}
			}
		}
	}
	for _, child := range node.Content {
		if err := checkIntegerFields(child); err != nil {
			return err
		}
	}
	return nil
}
