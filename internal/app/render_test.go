package app

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
)

func TestRenderFailureDoesNotSendPartialPage(t *testing.T) {
	component := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, err := io.WriteString(w, "partial page"); err != nil {
			return err
		}
		return errors.New("render failed")
	})
	a := &App{}
	w := httptest.NewRecorder()
	a.render(w, httptest.NewRequest("GET", "/", nil), component, 200)
	assert.Equal(t, 500, w.Code)
	assert.NotContains(t, w.Body.String(), "partial page")
}
