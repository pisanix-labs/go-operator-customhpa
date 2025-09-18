package prom

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strconv"
    "time"
)

// QueryInstant evaluates a PromQL instant query and returns the first scalar value.
// For simplicity, it assumes a vector/scalar result and returns 0 when empty.
func QueryInstant(ctx context.Context, baseURL, query string) (float64, error) {
    if baseURL == "" {
        return 0, fmt.Errorf("prometheusURL is required")
    }
    u, err := url.Parse(baseURL)
    if err != nil { return 0, err }
    u.Path = "/api/v1/query"
    q := u.Query()
    q.Set("query", query)
    u.RawQuery = q.Encode()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
    if err != nil { return 0, err }
    httpClient := &http.Client{ Timeout: 10 * time.Second }
    resp, err := httpClient.Do(req)
    if err != nil { return 0, err }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return 0, fmt.Errorf("prometheus http status %d", resp.StatusCode)
    }
    var pr promResp
    if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil { return 0, err }
    if pr.Status != "success" {
        return 0, fmt.Errorf("prometheus error: %s", pr.Error)
    }
    // Support scalar or vector (first sample)
    switch pr.Data.ResultType {
    case "scalar":
        if len(pr.Data.Result) >= 2 {
            return parseFloat(pr.Data.Result[1])
        }
    case "vector":
        if len(pr.Data.Result) > 0 {
            if s, ok := pr.Data.Result[0].(map[string]any)["value"]; ok {
                if arr, ok := s.([]any); ok && len(arr) >= 2 {
                    return parseFloat(arr[1])
                }
            }
        }
    }
    return 0, nil
}

func parseFloat(v any) (float64, error) {
    switch t := v.(type) {
    case string:
        return strconv.ParseFloat(t, 64)
    case float64:
        return t, nil
    default:
        return 0, fmt.Errorf("unexpected value type %T", v)
    }
}

// Minimal structures to unmarshal Prometheus response
type promResp struct {
    Status string   `json:"status"`
    Data   promData `json:"data"`
    Error  string   `json:"error"`
}

type promData struct {
    ResultType string      `json:"resultType"`
    Result     []any       `json:"result"`
}
