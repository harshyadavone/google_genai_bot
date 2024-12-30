package genai

import (
	"fmt"
	"log"
	"time"
)

func init() {
	log.SetFlags(0)
	log.SetPrefix("")
}

func logWithTime(format string, args ...any) {
	timeStapm := time.Now().Format("2006-01-02 15:04:05.000")
	log.Printf("[%s] %s", timeStapm, fmt.Sprintf(format, args...))
}
