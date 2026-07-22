package client

import "strings"

func appendSSEDataLine(buffer string, line string) string {
	data := strings.TrimPrefix(line[5:], " ")
	if buffer == "" {
		return data
	}
	return buffer + "\n" + data
}
