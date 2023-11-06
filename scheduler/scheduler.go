package scheduler

import (
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-co-op/gocron"
)

var scheduler *gocron.Scheduler

func GetScheduler() *gocron.Scheduler {
	if scheduler != nil {
		return scheduler
	}

	location, err := time.LoadLocation(os.Getenv("TZ"))
	if err != nil {
		log.Fatal("Error loading timezone", "error", err)
	}
	scheduler = gocron.NewScheduler(location)

	return scheduler
}
