package sync

import (
	"errors"
	"example/lastfm-spotify-syncer/config"
	lastFmApi "example/lastfm-spotify-syncer/lastfm/api"
	spotifyApi "example/lastfm-spotify-syncer/spotify/api"
	"fmt"
	"regexp"
	"time"

	"github.com/charmbracelet/log"
)

// transformStringForSpotify removes "'" "{" and "}" from a given string, as Spotify can't handle them in the track name when searching
func transformStringForSpotify(str string) string {
	// Define the pattern to match ', { and }
	re := regexp.MustCompile(`['{}]`)
	// Replace the matched patterns with an empty string
	result := re.ReplaceAllString(str, "")

	return result
}

// Sync the lastfm track data into a spotify playlist
func Sync(period string) error {
	var topTracksData *lastFmApi.TopTracks
	var err error

	conf, err := config.LoadConfig(false)
	if err != nil {
		log.Error("Error loading config", "err", err)
		return err
	}

	switch period {
	case "weekly":
		topTracksData, err = lastFmApi.GetTopTracks(period, conf.Config.Sync.Weekly.MaxTracks, conf.Auth.LastFM.Username)
	case "monthly":
		topTracksData, err = lastFmApi.GetTopTracks(period, conf.Config.Sync.Monthly.MaxTracks, conf.Auth.LastFM.Username)
	default:
		log.Error("Invalid frequency given", "freq", period)
		return errors.New("invalid period given")
	}
	if err != nil {
		log.Error("Unable to fetch from last fm api", "error", err)
		return err
	}
	spotifyUserData, err := spotifyApi.GetUser()
	if err != nil {
		log.Error("Unable to fetch from spotify api", "error", err)
		return err
	}

	var trackIds []string

	// Iterate and search for each track
	// TODO: concurrently?
	for _, v := range topTracksData.Toptracks.Track {
		trackName := v.Name
		artistName := v.Artist.Name
		log.Info("track data", "name", trackName, "artist", artistName)

		var searchData spotifyApi.Search
		searchQuery := fmt.Sprintf("artist: \"%s\" track: \"%s\"", artistName, trackName)
		searchQuery = transformStringForSpotify(searchQuery)
		log.Debug("search query string", "query", searchQuery)
		err := spotifyApi.Get(&searchData, "/search", map[string]string{
			"q":     searchQuery,
			"type":  "track",
			"limit": "1",
		})

		if err != nil {
			log.Error("error searching spotify", "error", err)
			continue
		}

		var items = searchData.Tracks.Items
		if len(items) == 0 {
			log.Warn("Spotify search returned no results for this track")
			continue
		}
		trackIds = append(trackIds, items[0].ID)
	}
	log.Info("track ids", "ids", trackIds)

	var playlistName string
	switch period {
	case "weekly":
		now := time.Now()
		sevenDaysAgo := now.AddDate(0, 0, -7)
		year := sevenDaysAgo.Year()
		playlistName = fmt.Sprintf("LastFM Top Tracks: %s-%s %d", sevenDaysAgo.Format("Jan 02"), now.Format("Jan 02"), year)
	case "monthly":
		currentTime := time.Now()
		firstDayOfCurrentMonth := time.Date(currentTime.Year(), currentTime.Month(), 1, 0, 0, 0, 0, currentTime.Location())
		lastDayOfPreviousMonth := firstDayOfCurrentMonth.Add(-time.Second)
		previousMonth := lastDayOfPreviousMonth.Month()
		year := lastDayOfPreviousMonth.Year()
		playlistName = fmt.Sprintf("LastFM Top Tracks: %s %d", previousMonth, year)
	}

	// Create a new playlist
	playlistData, err := spotifyApi.CreatePlaylist(spotifyUserData.ID, playlistName)
	if err != nil {
		log.Error("error creating playlist", "error", err)
		return err
	}
	log.Info("created playlist", "playlist", playlistData)

	// Add the tracks to the new playlist by uri
	_, err = spotifyApi.AddItemsToPlaylist(playlistData.ID, trackIds)
	if err != nil {
		log.Error("error adding items to playlist playlist", "error", err)
		return err // TODO: try delete the blank playlist here
	}

	log.Info("Populated playlist!")
	return nil
}
