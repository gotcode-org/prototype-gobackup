package main

import (
	"os"
	"regexp"
	"strings"
)

func main() {
	b, _ := os.ReadFile("internal/engine/server.go")
	str := string(b)
	
	re := regexp.MustCompile(`(?s)func \(s \*Server\) GetStatus\(ctx context\.Context, req \*pb\.StatusRequest\) \(\*pb\.StatusResponse, error\) \{.*?\}, nil\n\}`)
	
	matches := re.FindAllString(str, -1)
	if len(matches) >= 2 {
		str = strings.Replace(str, matches[0], "", 1)
	}

	os.WriteFile("internal/engine/server.go", []byte(str), 0644)
}
