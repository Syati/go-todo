package domain

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

type Admin struct {
	ID          int64
	Email, Role string
}

type Product struct {
	ID                        int64
	Name, Description, Status string
	TaxRate                   int
}

type SKU struct {
	ID, ProductID           int64
	Code, Size, Color       string
	PriceYen, StockQuantity int64
}

func (p Product) Validate() error {
	if strings.TrimSpace(p.Name) == "" || utf8.RuneCountInString(p.Name) > 100 {
		return errors.New("商品名は1〜100文字で入力してください。")
	}
	if utf8.RuneCountInString(p.Description) > 2000 {
		return errors.New("説明は2000文字以内で入力してください。")
	}
	if p.Status != "draft" && p.Status != "on_sale" && p.Status != "discontinued" {
		return errors.New("商品ステータスが不正です。")
	}
	if p.TaxRate != 8 && p.TaxRate != 10 {
		return errors.New("税率は8%または10%を指定してください。")
	}
	return nil
}

var skuCode = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{0,49}$`)

func (s *SKU) Normalize() {
	s.Code = strings.ToUpper(strings.TrimSpace(s.Code))
	s.Size = strings.TrimSpace(s.Size)
	s.Color = strings.TrimSpace(s.Color)
}

func (s SKU) Validate() error {
	if !skuCode.MatchString(s.Code) {
		return errors.New("SKUコードは50文字以内の英数字・ハイフンで入力してください（先頭は英数字）。")
	}
	if utf8.RuneCountInString(s.Size) > 30 || utf8.RuneCountInString(s.Color) > 30 {
		return errors.New("サイズ・カラーは30文字以内で入力してください。")
	}
	if s.PriceYen < 0 || s.PriceYen > 10000000 || s.StockQuantity < 0 {
		return errors.New("価格は0〜10,000,000円、在庫は0以上の整数で入力してください。")
	}
	return nil
}
