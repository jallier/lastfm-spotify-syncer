package scheduler

import (
	"errors"
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

// Start ALL jobs
func StartScheduler() {
	s := GetScheduler()
	s.PauseJobExecution(false)
	log.Info("Scheduler jobs running")
}

// Stop ALL jobs that aren't already running
func StopScheduler() {
	s := GetScheduler()
	s.PauseJobExecution(true)
	log.Info("Scheduler jobs paused")
}

// Start either the weekly or monthly job, depending on which tag given
// Tag values can be 'weekly' or 'monthly' or an error is returned
func StartJob(tag string) error {
	s := GetScheduler()
	var err error
	switch tag {
	case "weekly":
		err = startWeeklyJob(s)
	case "monthly":
		err = startMonthlyJob(s)
	default:
		err = errors.New("invalid tag given")
	}

	if err != nil {
		return err
	}

	return nil
}

// Stop either the weekly or monthly job, depending on which tag given
// Tag values can be 'weekly' or 'monthly' or an error is returned
func StopJob(tag string) error {
	s := GetScheduler()
	var err error
	switch tag {
	case "weekly":
		fallthrough
	case "monthly":
		err = s.RemoveByTag(tag)
	default:
		err = errors.New("invalid tag given")
	}
	if err != nil {
		return err
	}

	log.Info("Stopped job", "tag", tag)
	return nil
}

func startWeeklyJob(s *gocron.Scheduler) error {
	_, err := s.Every(1).Week().Tag("weekly").Do(func() {
		log.Info("Running weekly sync job...")
		sync.Sync("weekly")
		log.Info("Sync job complete")
	})
	if err != nil {
		log.Error("Error scheduling weekly job", "error", err)
		return err
	}

	log.Info("Weekly job scheduled")
	return nil
}

func startMonthlyJob(s *gocron.Scheduler) error {
	_, err := s.Every(1).Month(1).Tag("monthly").Do(func() {
		log.Info("Running monthly sync job...")
		sync.Sync("monthly")
		log.Info("Sync job complete")
	})
	if err != nil {
		log.Error("Error scheduling monthly job", "error", err)
		return err
	}

	log.Info("Monthly job scheduled")
	return nil
}

// Setup the scheduler and jobs for use later
// The jobs to enable must be given by passing in a slice of the tags to enable.
// These values must be 'weekly' or 'monthly'. Other values will be ignored. Duplicates will be ignored
func SetupSchedule(jobTags []string) error {
	s := GetScheduler()
	s.WaitForScheduleAll()
	s.TagsUnique()

	var err error
	for _, tag := range jobTags {
		switch tag {
		case "weekly":
			err = startWeeklyJob(s)
		case "monthly":
			err = startMonthlyJob(s)
		}
	}
	if err != nil {
		return err
	}

	s.StartAsync()
	return nil
}
