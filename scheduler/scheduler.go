package scheduler

import (
	"example/lastfm-spotify-syncer/sync"
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

func StartScheduler() {
	s := GetScheduler()
	s.PauseJobExecution(false)
	log.Info("Scheduler jobs running")
}

func StopScheduler() {
	s := GetScheduler()
	s.PauseJobExecution(true)
	log.Info("Scheduler jobs paused")
}

// Setup the scheduler and jobs for use later
func SetupSchedule() error {
	// setup the scheduler
	s := GetScheduler()
	s.WaitForScheduleAll()

	// Schedule weekly job - hardcoded for now
	_, err := s.Every(1).Week().Do(func() {
		log.Info("Running weekly sync job...")
		sync.Sync("weekly")
		log.Info("Sync job complete")
	})
	if err != nil {
		log.Error("Error scheduling weekly job", "error", err)
		return err
	}

	// Schedule monthly job - hardcoded for now
	_, err = s.Every(1).Month(1).Do(func() {
		log.Info("Running monthly sync job...")
		sync.Sync("monthly")
		log.Info("Sync job complete")
	})
	if err != nil {
		log.Error("Error scheduling monthly job", "error", err)
		return err
	}

	s.StartAsync()
	return nil
}
