package guest

import "path"

func matchesAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if pattern == value {
			return true
		}
		if ok, err := path.Match(pattern, value); err == nil && ok {
			return true
		}
	}
	return false
}
