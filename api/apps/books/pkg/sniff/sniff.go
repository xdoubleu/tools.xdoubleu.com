// Package sniff detects a payload's content type from its bytes.
package sniff

import "net/http"

// ContentType returns the MIME type of data's leading bytes.
func ContentType(data []byte) string {
	return http.DetectContentType(data)
}
