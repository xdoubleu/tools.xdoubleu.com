package oauth2as

// Config is what the authorization server needs from its host.
type Config struct {
	// HMACSecret keys token hashing and consent tokens.
	HMACSecret string
	// Issuer is the OAuth issuer URL.
	Issuer string
	// APIURL is the base URL of the resource server's own API.
	APIURL string
	// WebURL is the base URL of the web app serving the consent page.
	WebURL string
}
