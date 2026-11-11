package contracts

import "github.com/thanhmaitran/compatlab/internal/domain"

// matchTemplate delegates to domain.MatchPath.
func matchTemplate(template, concrete string) (int, bool) {
	return domain.MatchPath(template, concrete)
}
