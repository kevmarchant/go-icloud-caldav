package caldav

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func normalizeEventPath(eventPath, baseURL string) string {
	if !strings.HasPrefix(eventPath, "http://") && !strings.HasPrefix(eventPath, "https://") {
		if !strings.HasPrefix(eventPath, "/") {
			eventPath = "/" + eventPath
		}
		eventPath = baseURL + eventPath
	}
	return eventPath
}

func parseDurationNumber(numStr string) (int, error) {
	if numStr == "" {
		return 0, nil
	}
	var num int
	_, err := fmt.Sscanf(numStr, "%d", &num)
	return num, err
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("no response body")
	}
	return io.ReadAll(resp.Body)
}
