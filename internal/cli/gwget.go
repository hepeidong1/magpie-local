package cli

import (
	"net/http"

	"github.com/yetone/magpie/internal/access"
)

// gatewayGet asks the gateway at url, with the first enabled gateway key:
// the gateway answers only a request that carries one, this computer's
// probes included.
func gatewayGet(c *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if k := access.PrimarySecret(); k != "" {
		req.Header.Set("Authorization", "Bearer "+k)
	}
	return c.Do(req)
}
