// Package ai reads owner messages with Claude, falling back to the rule-based parser when no
// API key is set, the call fails, or the answer doesn't validate. The demo never depends on it.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/RostKostia/SKA/backend/internal/parse"
)

type Parser interface {
	Parse(ctx context.Context, text string) parse.Result
}

// Rules is the parser without AI.
type Rules struct{}

func (Rules) Parse(_ context.Context, text string) parse.Result { return parse.Detect(text) }

type Claude struct {
	APIKey  string
	Model   string // e.g. claude-haiku-5-5
	BaseURL string // https://api.anthropic.com
	HTTP    *http.Client
	Log     *slog.Logger
}

func (c *Claude) Parse(ctx context.Context, text string) parse.Result {
	if strings.TrimSpace(text) == "" {
		return parse.Detect(text)
	}
	res, err := c.call(ctx, text)
	if err != nil {
		c.Log.Warn("claude parse failed; using rules", "err", err)
		return parse.Detect(text)
	}
	return res
}

const systemPrompt = `You read short chat messages from the owner of a small business in Cyprus and record what they want, using the record_intent tool. Amounts are in euros; convert them to integer cents (€40.50 = 4050). Intents:
- create_invoice: they want to invoice a client ("Invoice Kostas €120", "Bill Elena 3 hours at 40"). Give the client's name exactly as written and one line per item: description, quantity, unit (hour, day, item or service) and unit_price_cents. "3 hours at €40" is quantity 3, unit hour, unit_price_cents 4000. If no amount is given, return no lines.
- provide_details: the message is mainly an email address (put it in email).
- remind: chase a client for payment (client), or everyone (all = true).
- forecast: questions about cash, balance, going negative, affordability.
- financing: they want money now, an advance or a loan.
- debtors: who owes them money.
- unknown: anything else.`

var toolSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"intent": map[string]any{"type": "string", "enum": []string{"create_invoice", "provide_details", "remind", "forecast", "financing", "debtors", "unknown"}},
		"client": map[string]any{"type": "string"},
		"all":    map[string]any{"type": "boolean"},
		"email":  map[string]any{"type": "string"},
		"lines": map[string]any{"type": "array", "items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"description":      map[string]any{"type": "string"},
				"quantity":         map[string]any{"type": "number"},
				"unit":             map[string]any{"type": "string", "enum": []string{"hour", "day", "item", "service"}},
				"unit_price_cents": map[string]any{"type": "integer"},
			},
			"required": []string{"description", "quantity", "unit", "unit_price_cents"},
		}},
	},
	"required": []string{"intent"},
}

type toolInput struct {
	Intent string `json:"intent"`
	Client string `json:"client"`
	All    bool   `json:"all"`
	Email  string `json:"email"`
	Lines  []struct {
		Description    string  `json:"description"`
		Quantity       float64 `json:"quantity"`
		Unit           string  `json:"unit"`
		UnitPriceCents int64   `json:"unit_price_cents"`
	} `json:"lines"`
}

func (c *Claude) call(ctx context.Context, text string) (parse.Result, error) {
	body, _ := json.Marshal(map[string]any{
		"model":       c.Model,
		"max_tokens":  512,
		"system":      systemPrompt,
		"tools":       []any{map[string]any{"name": "record_intent", "description": "Record what the owner wants.", "input_schema": toolSchema}},
		"tool_choice": map[string]any{"type": "tool", "name": "record_intent"},
		"messages":    []any{map[string]any{"role": "user", "content": text}},
	})
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return parse.Result{}, err
	}
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return parse.Result{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return parse.Result{}, fmt.Errorf("anthropic: %s: %s", resp.Status, truncate(string(raw), 300))
	}
	var out struct {
		Content []struct {
			Type  string          `json:"type"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return parse.Result{}, err
	}
	for _, b := range out.Content {
		if b.Type == "tool_use" && b.Name == "record_intent" {
			var in toolInput
			if err := json.Unmarshal(b.Input, &in); err != nil {
				return parse.Result{}, err
			}
			return toResult(in)
		}
	}
	return parse.Result{}, fmt.Errorf("anthropic: no record_intent tool call in the answer")
}

func toResult(in toolInput) (parse.Result, error) {
	res := parse.Result{Intent: parse.Intent(in.Intent), Source: "claude", Client: strings.TrimSpace(in.Client), All: in.All, Email: strings.TrimSpace(in.Email)}
	switch res.Intent {
	case parse.CreateInvoice:
		d := &parse.Draft{Client: res.Client}
		for _, l := range in.Lines {
			if l.Quantity <= 0 || l.UnitPriceCents <= 0 || l.Quantity > 1e6 {
				return res, fmt.Errorf("anthropic: invalid line %+v", l)
			}
			unit := l.Unit
			if unit != "hour" && unit != "day" && unit != "item" {
				unit = "service"
			}
			desc := strings.TrimSpace(l.Description)
			if desc == "" {
				desc = "Services"
			}
			d.Lines = append(d.Lines, parse.Line{Description: desc, QuantityMilli: int64(math.Round(l.Quantity * 1000)), Unit: unit, UnitPriceCents: l.UnitPriceCents})
		}
		res.Draft = d
		if d.Client == "" {
			res.Missing = "client"
		} else if len(d.Lines) == 0 {
			res.Missing = "amount"
		}
		res.Client = ""
	case parse.ProvideDetails:
		if res.Email == "" {
			return res, fmt.Errorf("anthropic: provide_details without email")
		}
	case parse.Remind:
		if res.Client == "" {
			res.All = true
		}
	case parse.Forecast, parse.Financing, parse.Debtors, parse.Unknown:
	default:
		return res, fmt.Errorf("anthropic: unknown intent %q", in.Intent)
	}
	return res, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
