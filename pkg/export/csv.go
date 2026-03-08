package export

import (
	"bytes"
	"encoding/csv"
	"strconv"

	"git-pulse/internal/dashboard"
)

func CSV(result dashboard.Result) (string, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)

	rows := [][]string{
		{"metric", "value"},
		{"commits", strconv.Itoa(result.Snapshot.Overview.CommitCount)},
		{"authors", strconv.Itoa(result.Snapshot.Overview.AuthorCount)},
		{"net_lines", strconv.Itoa(result.Snapshot.Overview.NetLines)},
		{"current_streak", strconv.Itoa(result.Snapshot.Overview.CurrentStreak)},
		{"longest_streak", strconv.Itoa(result.Snapshot.Overview.LongestStreak)},
	}

	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}
	writer.Flush()
	return buffer.String(), writer.Error()
}
