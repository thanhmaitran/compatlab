package commands

import (
	"encoding/json"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// marshalChanges renders raw (unclassified) changes as indented JSON.
func marshalChanges(changes []domain.ChangeRecord) ([]byte, error) {
	if changes == nil {
		changes = []domain.ChangeRecord{}
	}
	b, err := json.MarshalIndent(changes, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
