package utils

import "time"

func GetCurrentTimestamp() string {
	var timeLayout = "2006-01-02T15:04:05Z"
	return time.Now().Format(timeLayout)
}
