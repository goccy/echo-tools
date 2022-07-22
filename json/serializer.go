package json

import (
	"fmt"
	"io"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/labstack/echo/v4"
)

type Serializer struct{}

func NewSerializer() *Serializer {
	return &Serializer{}
}

func (s *Serializer) Serialize(c echo.Context, v interface{}, indent string) error {
	return json.NewEncoder(c.Response()).EncodeWithOption(
		v,
		json.UnorderedMap(),
		json.DisableHTMLEscape(),
	)
}

func (s *Serializer) Deserialize(c echo.Context, v interface{}) error {
	buf, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return fmt.Errorf("failed to read buffer: %w", err)
	}
	err = json.UnmarshalWithOption(buf, v,
		json.DecodeFieldPriorityFirstWin(),
	)
	if e, ok := err.(*json.UnmarshalTypeError); ok {
		return echo.NewHTTPError(
			http.StatusBadRequest,
			fmt.Sprintf(
				"Unmarshal type error: expected=%v, got=%v, field=%v, offset=%v",
				e.Type,
				e.Value,
				e.Field,
				e.Offset,
			),
		).SetInternal(err)
	} else if e, ok := err.(*json.SyntaxError); ok {
		return echo.NewHTTPError(
			http.StatusBadRequest,
			fmt.Sprintf(
				"Syntax error: offset=%v, error=%v",
				e.Offset,
				e.Error(),
			),
		).SetInternal(err)
	}
	return err
}
