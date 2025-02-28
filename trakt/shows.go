package trakt

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elgatito/elementum/cache"
	"github.com/elgatito/elementum/config"
	"github.com/elgatito/elementum/fanart"
	"github.com/elgatito/elementum/playcount"
	"github.com/elgatito/elementum/tmdb"
	"github.com/elgatito/elementum/util"
	"github.com/elgatito/elementum/xbmc"
	"github.com/jmcvetta/napping"
)

// Fill fanart from TMDB
func setShowFanart(show *Show) *Show {
	if show.Images == nil {
		show.Images = &Images{}
	}
	if show.Images.Poster == nil {
		show.Images.Poster = &Sizes{}
	}
	if show.Images.Thumbnail == nil {
		show.Images.Thumbnail = &Sizes{}
	}
	if show.Images.FanArt == nil {
		show.Images.FanArt = &Sizes{}
	}
	if show.Images.Banner == nil {
		show.Images.Banner = &Sizes{}
	}
	if show.Images.ClearArt == nil {
		show.Images.ClearArt = &Sizes{}
	}

	if show.IDs == nil || show.IDs.TMDB == 0 {
		return show
	}

	tmdbImages := tmdb.GetShowImages(show.IDs.TMDB)
	if tmdbImages == nil {
		return show
	}

	if len(tmdbImages.Posters) > 0 {
		posterImage := tmdb.ImageURL(tmdbImages.Posters[0].FilePath, "w1280")
		for _, image := range tmdbImages.Posters {
			if image.Iso639_1 == config.Get().Language {
				posterImage = tmdb.ImageURL(image.FilePath, "w1280")
			}
		}
		show.Images.Poster.Full = posterImage
		show.Images.Thumbnail.Full = posterImage
	}
	if len(tmdbImages.Backdrops) > 0 {
		backdropImage := tmdb.ImageURL(tmdbImages.Backdrops[0].FilePath, "w1280")
		for _, image := range tmdbImages.Backdrops {
			if image.Iso639_1 == config.Get().Language {
				backdropImage = tmdb.ImageURL(image.FilePath, "w1280")
			}
		}
		show.Images.FanArt.Full = backdropImage
		show.Images.Banner.Full = backdropImage
	}
	return show
}

func setShowsFanart(shows []*Shows) []*Shows {
	wg := sync.WaitGroup{}
	for i, show := range shows {
		wg.Add(1)
		go func(idx int, s *Shows) {
			defer wg.Done()
			shows[idx].Show = setShowFanart(s.Show)
		}(i, show)
	}
	wg.Wait()

	return shows
}

func setProgressShowsFanart(shows []*ProgressShow) []*ProgressShow {
	wg := sync.WaitGroup{}
	wg.Add(len(shows))
	for i, show := range shows {
		go func(idx int, s *ProgressShow) {
			defer wg.Done()
			if s != nil && s.Show != nil {
				shows[idx].Show = setShowFanart(s.Show)
			}
		}(i, show)
	}
	wg.Wait()
	return shows
}

func setCalendarShowsFanart(shows []*CalendarShow) []*CalendarShow {
	wg := sync.WaitGroup{}
	for i, show := range shows {
		wg.Add(1)
		go func(idx int, s *CalendarShow) {
			defer wg.Done()
			shows[idx].Show = setShowFanart(s.Show)
		}(i, show)
	}
	wg.Wait()

	return shows
}

// GetShow ...
func GetShow(ID string) (show *Show) {
	endPoint := fmt.Sprintf("shows/%s", ID)

	params := napping.Params{
		"extended": "full,images",
	}.AsUrlValues()

	cacheStore := cache.NewDBStore()
	key := fmt.Sprintf(cache.TraktShowKey, ID)
	if err := cacheStore.Get(key, &show); err != nil {
		resp, err := Get(endPoint, params)
		if err != nil {
			log.Error(err)
			xbmc.Notify("Elementum", fmt.Sprintf("Failed getting Trakt show (%s), check your logs.", ID), config.AddonIcon())
			return
		}
		if err := resp.Unmarshal(&show); err != nil {
			log.Warning(err)
		}

		cacheStore.Set(key, show, cache.TraktShowExpire)
	}

	return
}

// GetShowByTMDB ...
func GetShowByTMDB(tmdbID string) (show *Show) {
	endPoint := fmt.Sprintf("search/tmdb/%s?type=show", tmdbID)

	params := napping.Params{}.AsUrlValues()

	cacheStore := cache.NewDBStore()
	key := fmt.Sprintf(cache.TraktShowTMDBKey, tmdbID)
	if err := cacheStore.Get(key, &show); err != nil {
		resp, err := Get(endPoint, params)
		if err != nil {
			log.Error(err)
			xbmc.Notify("Elementum", "Failed getting Trakt show using TMDB ID, check your logs.", config.AddonIcon())
			return
		}

		var results ShowSearchResults
		if err := resp.Unmarshal(&results); err != nil {
			log.Warning(err)
		}
		if results != nil && len(results) > 0 && results[0].Show != nil {
			show = results[0].Show
		}
		cacheStore.Set(key, show, cache.TraktShowTMDBExpire)
	}
	return
}

// GetShowByTVDB ...
func GetShowByTVDB(tvdbID string) (show *Show) {
	endPoint := fmt.Sprintf("search/tvdb/%s?type=show", tvdbID)

	params := napping.Params{}.AsUrlValues()

	cacheStore := cache.NewDBStore()
	key := fmt.Sprintf(cache.TraktShowTVDBKey, tvdbID)
	if err := cacheStore.Get(key, &show); err != nil {
		resp, err := Get(endPoint, params)
		if err != nil {
			log.Error(err)
			xbmc.Notify("Elementum", "Failed getting Trakt show using TVDB ID, check your logs.", config.AddonIcon())
			return
		}
		if err := resp.Unmarshal(&show); err != nil {
			log.Warning(err)
		}
		cacheStore.Set(key, show, cache.TraktShowTVDBExpire)
	}
	return
}

// GetSeasonEpisodes ...
func GetSeasonEpisodes(showID, seasonNumber int) (episodes []*Episode) {
	endPoint := fmt.Sprintf("shows/%d/seasons/%d", showID, seasonNumber)
	params := napping.Params{"extended": "episodes,full"}.AsUrlValues()

	cacheStore := cache.NewDBStore()
	key := fmt.Sprintf(cache.TraktSeasonKey, showID, seasonNumber)
	if err := cacheStore.Get(key, &episodes); err != nil {
		resp, err := Get(endPoint, params)
		if err != nil {
			log.Error(err)
			return
		}
		if err := resp.Unmarshal(&episodes); err != nil {
			log.Warning(err)
		}
		cacheStore.Set(key, episodes, cache.TraktSeasonExpire)
	}
	return
}

// GetEpisode ...
func GetEpisode(showID, seasonNumber, episodeNumber int) (episode *Episode) {
	endPoint := fmt.Sprintf("shows/%d/seasons/%d/episodes/%d", showID, seasonNumber, episodeNumber)
	params := napping.Params{"extended": "full,images"}.AsUrlValues()

	cacheStore := cache.NewDBStore()
	key := fmt.Sprintf(cache.TraktEpisodeKey, showID, seasonNumber, episodeNumber)
	if err := cacheStore.Get(key, &episode); err != nil {
		resp, err := Get(endPoint, params)
		if err != nil {
			log.Error(err)
			return
		}
		if err := resp.Unmarshal(&episode); err != nil {
			log.Warning(err)
		}
		cacheStore.Set(key, episode, cache.TraktEpisodeExpire)
	}
	return
}

// GetEpisodeByID ...
func GetEpisodeByID(id string) (episode *Episode) {
	endPoint := fmt.Sprintf("search/trakt/%s?type=episode", id)

	params := napping.Params{}.AsUrlValues()

	cacheStore := cache.NewDBStore()
	key := fmt.Sprintf(cache.TraktEpisodeByIDKey, id)
	if err := cacheStore.Get(key, &episode); err != nil {
		resp, err := Get(endPoint, params)
		if err != nil {
			log.Error(err)
			xbmc.Notify("Elementum", "Failed getting Trakt episode, check your logs.", config.AddonIcon())
			return
		}
		if err := resp.Unmarshal(&episode); err != nil {
			log.Warning(err)
		}
		cacheStore.Set(key, episode, cache.TraktEpisodeByIDExpire)
	}
	return
}

// GetEpisodeByTMDB ...
func GetEpisodeByTMDB(tmdbID string) (episode *Episode) {
	endPoint := fmt.Sprintf("search/tmdb/%s?type=episode", tmdbID)

	params := napping.Params{}.AsUrlValues()

	cacheStore := cache.NewDBStore()
	key := fmt.Sprintf(cache.TraktEpisodeByTMDBKey, tmdbID)
	if err := cacheStore.Get(key, &episode); err != nil {
		resp, err := Get(endPoint, params)
		if err != nil {
			log.Error(err)
			xbmc.Notify("Elementum", "Failed getting Trakt episode using TMDB ID, check your logs.", config.AddonIcon())
			return
		}

		var results EpisodeSearchResults
		if err := resp.Unmarshal(&results); err != nil {
			log.Warning(err)
		}
		if results != nil && len(results) > 0 && results[0].Episode != nil {
			episode = results[0].Episode
		}
		cacheStore.Set(key, episode, cache.TraktEpisodeByTMDBExpire)
	}
	return
}

// GetEpisodeByTVDB ...
func GetEpisodeByTVDB(tvdbID string) (episode *Episode) {
	endPoint := fmt.Sprintf("search/tvdb/%s?type=episode", tvdbID)

	params := napping.Params{}.AsUrlValues()

	cacheStore := cache.NewDBStore()
	key := fmt.Sprintf(cache.TraktEpisodeByTVDBKey, tvdbID)
	if err := cacheStore.Get(key, &episode); err != nil {
		resp, err := Get(endPoint, params)
		if err != nil {
			log.Error(err)
			xbmc.Notify("Elementum", "Failed getting Trakt episode using TVDB ID, check your logs.", config.AddonIcon())
			return
		}
		if err := resp.Unmarshal(&episode); err != nil {
			log.Warning(err)
		}
		cacheStore.Set(key, episode, cache.TraktEpisodeByTVDBExpire)
	}
	return
}

// SearchShows ...
// TODO: Actually use this somewhere
func SearchShows(query string, page string) (shows []*Shows, err error) {
	endPoint := "search"

	params := napping.Params{
		"page":     page,
		"limit":    strconv.Itoa(config.Get().ResultsPerPage),
		"query":    query,
		"extended": "full,images",
	}.AsUrlValues()

	resp, err := Get(endPoint, params)

	if err != nil {
		return
	} else if resp.Status() != 200 {
		log.Error(err)
		return shows, fmt.Errorf("Bad status searching Trakt shows: %d", resp.Status())
	}

	if err := resp.Unmarshal(&shows); err != nil {
		log.Warning(err)
	}

	return
}

// TopShows ...
func TopShows(topCategory string, page string) (shows []*Shows, total int, err error) {
	endPoint := "shows/" + topCategory
	if topCategory == "recommendations" {
		endPoint = topCategory + "/shows"
	}

	resultsPerPage := config.Get().ResultsPerPage
	limit := resultsPerPage * PagesAtOnce
	pageInt, err := strconv.Atoi(page)
	if err != nil {
		return shows, 0, err
	}
	page = strconv.Itoa((pageInt-1)*resultsPerPage/limit + 1)
	params := napping.Params{
		"page":     page,
		"limit":    strconv.Itoa(limit),
		"extended": "full,images",
	}.AsUrlValues()

	cacheStore := cache.NewDBStore()
	key := fmt.Sprintf(cache.TraktShowsByCategoryKey, topCategory, page)
	totalKey := fmt.Sprintf(cache.TraktShowsByCategoryTotalKey, topCategory)
	if err := cacheStore.Get(key, &shows); err != nil || len(shows) == 0 {
		var resp *napping.Response
		var err error

		if !config.Get().TraktAuthorized {
			resp, err = Get(endPoint, params)
		} else {
			resp, err = GetWithAuth(endPoint, params)
		}

		if err != nil {
			return shows, 0, err
		} else if resp.Status() != 200 {
			return shows, 0, fmt.Errorf("Bad status getting top %s Trakt shows: %d", topCategory, resp.Status())
		}

		if topCategory == "popular" || topCategory == "recommendations" {
			var showList []*Show
			if errUnm := resp.Unmarshal(&showList); errUnm != nil {
				return shows, 0, errUnm
			}

			showListing := make([]*Shows, 0)
			for _, show := range showList {
				showItem := Shows{
					Show: show,
				}
				showListing = append(showListing, &showItem)
			}
			shows = showListing
		} else {
			if errUnm := resp.Unmarshal(&shows); errUnm != nil {
				log.Warning(errUnm)
			}
		}

		pagination := getPagination(resp.HttpResponse().Header)
		total = pagination.ItemCount
		if err != nil {
			log.Warning(err)
		} else {
			cacheStore.Set(totalKey, total, cache.TraktShowsByCategoryTotalExpire)
		}

		cacheStore.Set(key, shows, cache.TraktShowsByCategoryExpire)
	} else {
		if err := cacheStore.Get(totalKey, &total); err != nil {
			total = -1
		}
	}

	return
}

// WatchlistShows ...
func WatchlistShows(isUpdateNeeded bool) (shows []*Shows, err error) {
	if err := Authorized(); err != nil {
		return shows, err
	}

	endPoint := "sync/watchlist/shows"

	params := napping.Params{
		"extended": "full,images",
	}.AsUrlValues()

	cacheStore := cache.NewDBStore()

	if !isUpdateNeeded {
		if err := cacheStore.Get(cache.TraktShowsWatchlistKey, &shows); err == nil {
			return shows, nil
		}
	}

	resp, err := GetWithAuth(endPoint, params)

	if err != nil {
		return shows, err
	} else if resp.Status() != 200 {
		log.Error(err)
		return shows, fmt.Errorf("Bad status getting Trakt watchlist for shows: %d", resp.Status())
	}

	var watchlist []*WatchlistShow
	if err := resp.Unmarshal(&watchlist); err != nil {
		log.Warning(err)
	}

	showListing := make([]*Shows, 0)
	for _, show := range watchlist {
		showItem := Shows{
			Show: show.Show,
		}
		showListing = append(showListing, &showItem)
	}
	shows = showListing

	cacheStore.Set(cache.TraktShowsWatchlistKey, &shows, cache.TraktShowsWatchlistExpire)
	return
}

// PreviousWatchlistShows ...
func PreviousWatchlistShows() (shows []*Shows, err error) {
	err = cache.
		NewDBStore().
		Get(cache.TraktShowsWatchlistKey, &shows)

	return shows, err
}

// CollectionShows ...
func CollectionShows(isUpdateNeeded bool) (shows []*Shows, err error) {
	if err := Authorized(); err != nil {
		return shows, err
	}

	endPoint := "sync/collection/shows"

	params := napping.Params{
		"extended": "full,images",
	}.AsUrlValues()

	cacheStore := cache.NewDBStore()

	if !isUpdateNeeded {
		if err := cacheStore.Get(cache.TraktShowsCollectionKey, &shows); err == nil {
			return shows, nil
		}
	}

	resp, err := GetWithAuth(endPoint, params)

	if err != nil {
		return shows, err
	} else if resp.Status() != 200 {
		return shows, fmt.Errorf("Bad status getting Trakt collection for shows: %d", resp.Status())
	}

	var collection []*CollectionShow
	if err := resp.Unmarshal(&collection); err != nil {
		log.Warning(err)
	}

	showListing := make([]*Shows, 0, len(collection))
	for _, show := range collection {
		showItem := Shows{
			Show: show.Show,
		}
		showListing = append(showListing, &showItem)
	}

	cacheStore.Set(cache.TraktShowsCollectionKey, &showListing, cache.TraktShowsCollectionExpire)
	return showListing, err
}

// PreviousCollectionShows ...
func PreviousCollectionShows() (shows []*Shows, err error) {
	err = cache.
		NewDBStore().
		Get(cache.TraktShowsCollectionKey, &shows)

	return shows, err
}

// ListItemsShows ...
func ListItemsShows(user string, listID string, isUpdateNeeded bool) (shows []*Shows, err error) {
	if user == "" || user == "id" {
		user = config.Get().TraktUsername
	}

	endPoint := fmt.Sprintf("users/%s/lists/%s/items/shows", user, listID)

	params := napping.Params{}.AsUrlValues()

	var resp *napping.Response

	cacheStore := cache.NewDBStore()
	key := fmt.Sprintf(cache.TraktShowsListKey, listID)

	if !isUpdateNeeded {
		if err := cacheStore.Get(key, &shows); err == nil {
			return shows, nil
		}
	}

	var errGet error
	if !config.Get().TraktAuthorized {
		resp, errGet = Get(endPoint, params)
	} else {
		resp, errGet = GetWithAuth(endPoint, params)
	}

	if errGet != nil || resp.Status() != 200 {
		return shows, errGet
	}

	var list []*ListItem
	if err = resp.Unmarshal(&list); err != nil {
		log.Warning(err)
	}

	showListing := make([]*Shows, 0)
	for _, show := range list {
		if show.Show == nil {
			continue
		}
		showItem := Shows{
			Show: show.Show,
		}
		showListing = append(showListing, &showItem)
	}
	shows = showListing

	cacheStore.Set(key, &shows, cache.TraktShowsListExpire)
	return shows, err
}

// PreviousListItemsShows ...
func PreviousListItemsShows(listID string) (shows []*Shows, err error) {
	cacheStore := cache.NewDBStore()
	key := fmt.Sprintf(cache.TraktShowsListKey, listID)
	err = cacheStore.Get(key, &shows)

	return
}

// CalendarShows ...
func CalendarShows(endPoint string, page string) (shows []*CalendarShow, total int, err error) {
	resultsPerPage := config.Get().ResultsPerPage
	limit := resultsPerPage * PagesAtOnce
	pageInt, err := strconv.Atoi(page)
	if err != nil {
		return shows, 0, err
	}
	page = strconv.Itoa((pageInt-1)*resultsPerPage/limit + 1)
	params := napping.Params{
		"page":     page,
		"limit":    strconv.Itoa(limit),
		"extended": "full,images",
	}.AsUrlValues()

	cacheStore := cache.NewDBStore()
	endPointKey := strings.Replace(endPoint, "/", ".", -1)
	key := fmt.Sprintf(cache.TraktShowsCalendarKey, endPointKey, page)
	totalKey := fmt.Sprintf(cache.TraktShowsCalendarTotalKey, endPointKey)
	if err := cacheStore.Get(key, &shows); err != nil {
		resp, err := GetWithAuth("calendars/"+endPoint, params)

		if err != nil {
			return shows, 0, err
		} else if resp.Status() != 200 {
			return shows, 0, fmt.Errorf("Bad status getting %s Trakt shows: %d", endPoint, resp.Status())
		}

		if errUnm := resp.Unmarshal(&shows); errUnm != nil {
			log.Warning(errUnm)
		}

		pagination := getPagination(resp.HttpResponse().Header)
		total = pagination.ItemCount
		if err != nil {
			total = -1
		} else {
			cacheStore.Set(totalKey, total, cache.TraktShowsCalendarTotalExpire)
		}

		cacheStore.Set(key, &shows, cache.TraktShowsCalendarExpire)
	} else {
		if err := cacheStore.Get(totalKey, &total); err != nil {
			total = -1
		}
	}

	return
}

// WatchedShows ...
func WatchedShows(isUpdateNeeded bool) ([]*WatchedShow, error) {
	var shows []*WatchedShow
	err := Request(
		"sync/watched/shows",
		napping.Params{"extended": "full,images"},
		true,
		isUpdateNeeded,
		cache.TraktShowsWatchedKey,
		cache.TraktShowsWatchedExpire,
		&shows,
	)

	if len(shows) != 0 {
		cache.
			NewDBStore().
			Set(cache.TraktShowsWatchedKey, &shows, cache.TraktShowsWatchedExpire)
	}

	return shows, err
}

// PreviousWatchedShows ...
func PreviousWatchedShows() (shows []*WatchedShow, err error) {
	err = cache.
		NewDBStore().
		Get(cache.TraktShowsWatchedKey, &shows)

	return
}

// PausedShows ...
func PausedShows(isUpdateNeeded bool) ([]*PausedEpisode, error) {
	var shows []*PausedEpisode
	err := Request(
		"sync/playback/episodes",
		napping.Params{
			"extended": "full",
		},
		true,
		isUpdateNeeded,
		cache.TraktShowsPausedKey,
		cache.TraktShowsPausedExpire,
		&shows,
	)

	return shows, err
}

// WatchedShowsProgress ...
func WatchedShowsProgress() (shows []*ProgressShow, err error) {
	if errAuth := Authorized(); errAuth != nil {
		return nil, errAuth
	}

	cacheStore := cache.NewDBStore()

	lastActivities, err := GetLastActivities()
	if err != nil {
		log.Warningf("Cannot get activities: %s", err)
		return nil, err
	}
	var previousActivities UserActivities
	_ = cacheStore.Get(cache.TraktActivitiesKey, &previousActivities)

	// If last watched time was changed - we should get fresh Watched shows list
	isRefresh := !lastActivities.Episodes.WatchedAt.Equal(previousActivities.Episodes.WatchedAt)
	watchedShows, errWatched := WatchedShows(isRefresh)
	if errWatched != nil {
		log.Errorf("Error getting the watched shows: %v", errWatched)
		return nil, errWatched
	}

	params := napping.Params{
		"hidden":         "false",
		"specials":       "false",
		"count_specials": "false",
	}.AsUrlValues()

	showsList := make([]*ProgressShow, len(watchedShows))
	watchedProgressShows := make([]*WatchedProgressShow, len(watchedShows))

	var wg sync.WaitGroup
	wg.Add(len(watchedShows))
	for i, show := range watchedShows {
		go func(idx int, show *WatchedShow) {
			var watchedProgressShow *WatchedProgressShow
			var cachedWatchedAt time.Time

			defer func() {
				cacheStore.Set(fmt.Sprintf(cache.TraktWatchedShowsProgressWatchedKey, show.Show.IDs.Trakt), show.LastWatchedAt, cache.TraktWatchedShowsProgressWatchedExpire)

				watchedProgressShows[idx] = watchedProgressShow

				if watchedProgressShow != nil && watchedProgressShow.NextEpisode != nil && watchedProgressShow.NextEpisode.Number != 0 && watchedProgressShow.NextEpisode.Season != 0 {
					showsList[idx] = &ProgressShow{
						Show:    show.Show,
						Episode: watchedProgressShow.NextEpisode,
					}
				}
				wg.Done()
			}()

			if err := cacheStore.Get(fmt.Sprintf(cache.TraktWatchedShowsProgressWatchedKey, show.Show.IDs.Trakt), &cachedWatchedAt); err == nil && show.LastWatchedAt.Equal(cachedWatchedAt) {
				if err := cacheStore.Get(fmt.Sprintf(cache.TraktWatchedShowsProgressKey, show.Show.IDs.Trakt), &watchedProgressShow); err == nil {
					return
				}
			}

			endPoint := fmt.Sprintf("shows/%d/progress/watched", show.Show.IDs.Trakt)
			resp, err := GetWithAuth(endPoint, params)
			if err != nil {
				log.Errorf("Error getting endpoint %s for show '%d': %#v", endPoint, show.Show.IDs.Trakt, err)
				return
			} else if resp.Status() != 200 {
				log.Errorf("Got %d response status getting endpoint %s for show '%d'", resp.Status(), endPoint, show.Show.IDs.Trakt)
				return
			}
			if err := resp.Unmarshal(&watchedProgressShow); err != nil {
				log.Warningf("Can't unmarshal response: %#v", err)
			}

			cacheStore.Set(fmt.Sprintf(cache.TraktWatchedShowsProgressKey, show.Show.IDs.Trakt), &watchedProgressShow, cache.TraktWatchedShowsProgressExpire)
		}(i, show)
	}
	wg.Wait()

	for _, s := range showsList {
		if s != nil {
			shows = append(shows, s)
		}
	}

	return
}

// ToListItem ...
func (show *Show) ToListItem() (item *xbmc.ListItem) {
	if !config.Get().ForceUseTrakt && show.IDs.TMDB != 0 {
		tmdbID := strconv.Itoa(show.IDs.TMDB)
		if tmdbShow := tmdb.GetShowByID(tmdbID, config.Get().Language); tmdbShow != nil {
			item = tmdbShow.ToListItem()
		}
	}
	if item == nil {
		show = setShowFanart(show)
		item = &xbmc.ListItem{
			Label: show.Title,
			Info: &xbmc.ListItemInfo{
				Count:         rand.Int(),
				Title:         show.Title,
				OriginalTitle: show.Title,
				Year:          show.Year,
				Genre:         strings.Title(strings.Join(show.Genres, " / ")),
				Plot:          show.Overview,
				PlotOutline:   show.Overview,
				Rating:        show.Rating,
				Votes:         strconv.Itoa(show.Votes),
				Duration:      show.Runtime * 60,
				MPAA:          show.Certification,
				Code:          show.IDs.IMDB,
				IMDBNumber:    show.IDs.IMDB,
				Trailer:       util.TrailerURL(show.Trailer),
				PlayCount:     playcount.GetWatchedShowByTMDB(show.IDs.TMDB).Int(),
				DBTYPE:        "tvshow",
				Mediatype:     "tvshow",
			},
			Art: &xbmc.ListItemArt{
				TvShowPoster: show.Images.Poster.Full,
				Poster:       show.Images.Poster.Full,
				FanArt:       show.Images.FanArt.Full,
				Banner:       show.Images.Banner.Full,
				Thumbnail:    show.Images.Thumbnail.Full,
				ClearArt:     show.Images.ClearArt.Full,
			},
			Thumbnail: show.Images.Poster.Full,
		}
	}

	item.Thumbnail = item.Art.Poster
	// item.Art.Thumbnail = item.Art.Poster

	// if fa := fanart.GetShow(util.StrInterfaceToInt(show.IDs.TVDB)); fa != nil {
	// 	item.Art = fa.ToListItemArt(item.Art)
	// 	item.Thumbnail = item.Art.Thumbnail
	// }

	if len(item.Info.Trailer) == 0 {
		item.Info.Trailer = util.TrailerURL(show.Trailer)
	}

	return
}

// ToListItem ...
func (episode *Episode) ToListItem(show *Show) *xbmc.ListItem {
	episodeLabel := episode.Title
	if config.Get().AddEpisodeNumbers {
		episodeLabel = fmt.Sprintf("%dx%02d %s", episode.Season, episode.Number, episode.Title)
	}

	runtime := 1800
	if show.Runtime > 0 {
		runtime = show.Runtime
	}

	show = setShowFanart(show)
	item := &xbmc.ListItem{
		Label:  episodeLabel,
		Label2: fmt.Sprintf("%f", episode.Rating),
		Info: &xbmc.ListItemInfo{
			Count:         rand.Int(),
			Title:         episodeLabel,
			OriginalTitle: episode.Title,
			Season:        episode.Season,
			Episode:       episode.Number,
			TVShowTitle:   show.Title,
			Plot:          episode.Overview,
			PlotOutline:   episode.Overview,
			Rating:        episode.Rating,
			Aired:         episode.FirstAired,
			Duration:      runtime,
			Code:          show.IDs.IMDB,
			IMDBNumber:    show.IDs.IMDB,
			PlayCount:     playcount.GetWatchedEpisodeByTMDB(show.IDs.TMDB, episode.Season, episode.Number).Int(),
			DBTYPE:        "episode",
			Mediatype:     "episode",
		},
		Art: &xbmc.ListItemArt{
			TvShowPoster: show.Images.Poster.Full,
			Poster:       show.Images.Poster.Full,
			FanArt:       show.Images.FanArt.Full,
			Banner:       show.Images.Banner.Full,
			Thumbnail:    show.Images.Thumbnail.Full,
			ClearArt:     show.Images.ClearArt.Full,
		},
		Thumbnail: show.Images.Poster.Full,
	}

	item.Info.Genre = strings.Join(show.Genres, " / ")

	if config.Get().UseFanartTv {
		if fa := fanart.GetShow(util.StrInterfaceToInt(show.IDs.TVDB)); fa != nil {
			item.Art = fa.ToEpisodeListItemArt(episode.Season, item.Art)
		}
	}

	if episode.Images != nil && episode.Images.ScreenShot.Full != "" {
		item.Art.FanArt = episode.Images.ScreenShot.Full
		item.Art.Thumbnail = episode.Images.ScreenShot.Full
		item.Art.Poster = episode.Images.ScreenShot.Full
		item.Thumbnail = episode.Images.ScreenShot.Full
	} else if epi := tmdb.GetEpisode(show.IDs.TMDB, episode.Season, episode.Number, config.Get().Language); epi != nil && epi.StillPath != "" {
		item.Art.FanArt = tmdb.ImageURL(epi.StillPath, "w1280")
		item.Art.Thumbnail = tmdb.ImageURL(epi.StillPath, "w1280")
		item.Art.Poster = tmdb.ImageURL(epi.StillPath, "w1280")
		item.Thumbnail = tmdb.ImageURL(epi.StillPath, "w1280")
	}

	return item
}
