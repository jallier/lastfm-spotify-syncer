package api

type TopTracks struct {
	Toptracks struct {
		Attr struct {
			Page       string `json:"page"`
			Total      string `json:"total"`
			User       string `json:"user"`
			PerPage    string `json:"perPage"`
			TotalPages string `json:"totalPages"`
		} `json:"@attr"`
		Track []struct {
			Attr struct {
				Rank string `json:"rank"`
			} `json:"@attr"`
			Duration  string `json:"duration"`
			Playcount string `json:"playcount"`
			Artist    struct {
				URL  string `json:"url"`
				Name string `json:"name"`
				Mbid string `json:"mbid"`
			} `json:"artist"`
			Image []struct {
				Size string `json:"size"`
				Text string `json:"#text"`
			} `json:"image"`
			Streamable struct {
				Fulltrack string `json:"fulltrack"`
				Text      string `json:"#text"`
			} `json:"streamable"`
			Mbid string `json:"mbid"`
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"track"`
	} `json:"toptracks"`
}

type TrackInfo struct {
	Track struct {
		Name       string `json:"name"`
		Mbid       string `json:"mbid"`
		URL        string `json:"url"`
		Duration   string `json:"duration"`
		Streamable struct {
			Text      string `json:"#text"`
			Fulltrack string `json:"fulltrack"`
		} `json:"streamable"`
		Listeners string `json:"listeners"`
		Playcount string `json:"playcount"`
		Artist    struct {
			Name string `json:"name"`
			Mbid string `json:"mbid"`
			URL  string `json:"url"`
		} `json:"artist"`
		Album struct {
			Artist string `json:"artist"`
			Title  string `json:"title"`
			Mbid   string `json:"mbid"`
			URL    string `json:"url"`
			Image  []struct {
				Text string `json:"#text"`
				Size string `json:"size"`
			} `json:"image"`
			Attr struct {
				Position string `json:"position"`
			} `json:"@attr"`
		} `json:"album"`
		Userplaycount string `json:"userplaycount"`
		Userloved     string `json:"userloved"`
		Toptags       struct {
			Tag []struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"tag"`
		} `json:"toptags"`
		Wiki struct {
			Published string `json:"published"`
			Summary   string `json:"summary"`
			Content   string `json:"content"`
		} `json:"wiki"`
	} `json:"track"`
}
