package main

import (
	"time"

	"github.com/elgatito/elementum/repository"
	"github.com/elgatito/elementum/xbmc"
)

func checkRepository() bool {
	if xbmc.IsAddonInstalled("repository.elementum") {
		if !xbmc.IsAddonEnabled("repository.elementum") {
			xbmc.SetAddonEnabled("repository.elementum", true)
		}
		return true
	}

	log.Info("Creating Elementum repository add-on...")
	if err := repository.MakeElementumRepositoryAddon(); err != nil {
		log.Errorf("Unable to create repository add-on: %s", err)
		return false
	}

	xbmc.UpdateLocalAddons()
	for _, addon := range xbmc.GetAddons("xbmc.addon.repository", "unknown", "all", []string{"name", "version", "enabled"}).Addons {
		if addon.ID == "repository.elementum" && addon.Enabled == true {
			log.Info("Found enabled Elementum repository add-on")
			return false
		}
	}
	log.Info("Elementum repository not installed, installing...")
	xbmc.InstallAddon("repository.elementum")
	for timeout := 0; timeout < 10; timeout++ {
		if xbmc.IsAddonInstalled("repository.elementum") {
			break
		}
		log.Info("Sleeping 1 second while waiting for Elementum repository add-on to be installed")
		time.Sleep(1 * time.Second)
	}
	xbmc.SetAddonEnabled("repository.elementum", true)
	xbmc.UpdateLocalAddons()
	xbmc.UpdateAddonRepos()

	return true
}
