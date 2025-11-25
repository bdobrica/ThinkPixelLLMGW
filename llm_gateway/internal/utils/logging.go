package utils

import "log"

// LogError logs an error message
func LogError(err error) {
	if err != nil {
		log.Println("Error:", err)
	}
}
