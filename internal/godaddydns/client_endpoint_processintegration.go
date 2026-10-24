//go:build processintegration

package godaddydns

import (
	"net/url"
	"os"
)

const providerProductionBase = "https://process-test.invalid/v3/domains"

var providerOTEBase = processIntegrationProviderBase()

func processIntegrationProviderBase() string {
	value := os.Getenv("GOTTH_PROCESS_TEST_PROVIDER_BASE")
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" || parsed.Path != "/v3/domains" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		panic("exact loopback process-test provider base required")
	}
	return value
}
