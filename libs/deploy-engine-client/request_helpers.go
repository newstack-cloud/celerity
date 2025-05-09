package deployengine

import "net/http"

func attachHeaders(
	req *http.Request,
	headers map[string]string,
) {
	for headerName, value := range headers {
		req.Header.Set(headerName, value)
	}
}
