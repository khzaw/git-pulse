package export

import (
	"encoding/json"

	"git-pulse/internal/dashboard"
)

func JSON(result dashboard.Result) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}
