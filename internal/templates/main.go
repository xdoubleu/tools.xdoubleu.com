package templates

import (
	"embed"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"time"

	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
	"tools.xdoubleu.com/internal/config"
)

//go:embed html/shared/*.html
var sharedFS embed.FS

// RenderError renders the shared error.html page with the given HTTP status
// and message.
func RenderError(
	tpl *template.Template,
	w http.ResponseWriter,
	status int,
	message string,
) {
	w.WriteHeader(status)
	tpltools.RenderWithPanic(tpl, w, "error.html", map[string]any{
		"Status":  status,
		"Title":   http.StatusText(status),
		"Message": message,
	})
}

const eighths = 8

//nolint:gochecknoglobals //package-level lookup table, not mutable state
var fractionSymbols = map[int]string{
	0: "",
	1: "⅛",
	2: "¼",
	3: "⅜",
	4: "½",
	5: "⅝",
	6: "¾",
	7: "⅞",
}

// ToFraction converts a float64 to a Unicode cooking fraction string (nearest 1/8th).
func ToFraction(f float64) string {
	if f <= 0 {
		return "0"
	}
	whole := int(math.Floor(f))
	nearest := int(math.Round((f - float64(whole)) * eighths))
	if nearest == eighths {
		whole++
		nearest = 0
	}
	fracStr := fractionSymbols[nearest]
	if whole == 0 {
		if fracStr == "" {
			return "0"
		}
		return fracStr
	}
	return fmt.Sprintf("%d%s", whole, fracStr)
}

func LoadShared(cfg config.Config) *template.Template {
	return template.Must(template.New("shared").Funcs(template.FuncMap{
		"release": func() string {
			return cfg.Release[0:7]
		},
		"copyrightYear": func() int {
			return time.Now().Year()
		},
		"toFraction": ToFraction,
		"humanDate": func(t *time.Time) string {
			if t == nil {
				return ""
			}
			now := time.Now()
			today := time.Date(
				now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local,
			)
			d := time.Date(
				t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local,
			)
			//nolint:mnd // 24h/day, 7 days/week are fixed constants
			days := int(math.Round(d.Sub(today).Hours() / 24))
			switch days {
			case -1:
				return "Yesterday"
			case 0:
				return "Today"
			case 1:
				return "Tomorrow"
			default:
				if days > 1 && days < 7 {
					return d.Format("Mon")
				}
				return d.Format("2 Jan")
			}
		},
		"isOverdue": func(t *time.Time) bool {
			if t == nil {
				return false
			}
			now := time.Now()
			today := time.Date(
				now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local,
			)
			d := time.Date(
				t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local,
			)
			return d.Before(today)
		},
		"dict": func(keysAndValues ...any) (map[string]any, error) {
			const pairSize = 2
			if len(keysAndValues)%pairSize != 0 {
				return nil, fmt.Errorf("dict: odd number of arguments")
			}
			m := make(map[string]any, len(keysAndValues)/pairSize)
			for i := 0; i < len(keysAndValues); i += pairSize {
				key, ok := keysAndValues[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict: key must be a string")
				}
				m[key] = keysAndValues[i+1]
			}
			return m, nil
		},
	}).ParseFS(
		sharedFS,
		"html/shared/*.html",
	))
}
