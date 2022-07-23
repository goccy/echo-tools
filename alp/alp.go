package alp

import "strings"

// ConvertEchoRoutes convert echo framework's routes format
func ConvertEchoRoutes(routes []string) string {
	matchingGroup := make([]string, 0, len(routes))

	for _, s := range routes {
		found := false
		for i := 1; i < len(s); i++ {
			if i < len(s)-1 && s[i] == '/' && s[i+1] == ':' {
				next := len(s)
				for j := i + 2; j < len(s); j++ {
					if s[j] == '/' {
						next = j
						break
					}
				}
				matchingGroup = append(matchingGroup, s[:i+1]+".+"+s[next:])
				found = true
			}
		}
		if !found {
			matchingGroup = append(matchingGroup, s)
		}
	}

	ret := strings.Join(matchingGroup, ",")
	return ret
}
