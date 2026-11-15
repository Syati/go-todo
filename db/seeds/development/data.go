package development

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"go.yaml.in/yaml/v3"
)

//go:embed data/*.yml
var files embed.FS

type seedData struct {
	Admins   adminData
	Products []productSeed
}

type adminData struct {
	Password string      `yaml:"password"`
	Admins   []adminSeed `yaml:"admins"`
}

type adminSeed struct {
	Email   string `yaml:"email"`
	Role    string `yaml:"role"`
	Deleted bool   `yaml:"deleted"`
}

type productSeed struct {
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Status      string    `yaml:"status"`
	TaxRate     int       `yaml:"tax_rate"`
	SKUs        []skuSeed `yaml:"skus"`
}

type skuSeed struct {
	Code          string `yaml:"code"`
	Size          string `yaml:"size"`
	Color         string `yaml:"color"`
	PriceYen      int64  `yaml:"price_yen"`
	StockQuantity int64  `yaml:"stock_quantity"`
}

func loadData(source fs.FS) (seedData, error) {
	var data seedData
	if err := readYAML(source, "data/admins.yml", &data.Admins); err != nil {
		return seedData{}, err
	}
	var catalog struct {
		Products []productSeed `yaml:"products"`
	}
	if err := readYAML(source, "data/products.yml", &catalog); err != nil {
		return seedData{}, err
	}
	data.Products = catalog.Products
	if err := validateData(&data); err != nil {
		return seedData{}, err
	}
	return data, nil
}

func readYAML(source fs.FS, name string, destination any) error {
	contents, err := fs.ReadFile(source, name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return fmt.Errorf("%s: YAML ドキュメントは1つだけ指定してください", name)
	}
	return nil
}
