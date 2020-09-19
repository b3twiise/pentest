// Copyright 2017 Jeff Foley. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package sources

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/OWASP/Amass/amass/core"
	"github.com/OWASP/Amass/amass/utils"
)

// Shodan is the Service that handles access to the Shodan data source.
type Shodan struct {
	core.BaseService

	API        *core.APIKey
	SourceType string
	RateLimit  time.Duration
}

// NewShodan returns he object initialized, but not yet started.
func NewShodan(config *core.Config, bus *core.EventBus) *Shodan {
	s := &Shodan{
		SourceType: core.API,
		RateLimit:  time.Second,
	}

	s.BaseService = *core.NewBaseService(s, "Shodan", config, bus)
	return s
}

// OnStart implements the Service interface
func (s *Shodan) OnStart() error {
	s.BaseService.OnStart()

	s.API = s.Config().GetAPIKey(s.String())
	if s.API == nil || s.API.Key == "" {
		s.Config().Log.Printf("%s: API key data was not provided", s.String())
	}
	go s.startRootDomains()
	return nil
}

func (s *Shodan) startRootDomains() {
	// Look at each domain provided by the config
	for _, domain := range s.Config().Domains() {
		s.executeQuery(domain)
		// Honor the rate limit
		time.Sleep(s.RateLimit)
	}
}

func (s *Shodan) executeQuery(domain string) {
	if s.API == nil || s.API.Key == "" {
		return
	}

	url := s.restURL(domain)
	headers := map[string]string{"Content-Type": "application/json"}
	page, err := utils.RequestWebPage(url, nil, headers, "", "")
	if err != nil {
		s.Config().Log.Printf("%s: %s: %v", s.String(), url, err)
		return
	}
	// Extract the subdomain names from the REST API results
	var m struct {
		Matches []struct {
			Hostnames []string `json:"hostnames"`
		} `json:"matches"`
	}
	if err := json.Unmarshal([]byte(page), &m); err != nil {
		return
	}

	s.SetActive()
	re := s.Config().DomainRegex(domain)
	for _, match := range m.Matches {
		for _, host := range match.Hostnames {
			if !re.MatchString(host) {
				continue
			}
			s.Bus().Publish(core.NewNameTopic, &core.Request{
				Name:   host,
				Domain: domain,
				Tag:    s.SourceType,
				Source: s.String(),
			})
		}
	}
}

func (s *Shodan) restURL(domain string) string {
	return fmt.Sprintf("https://api.shodan.io/shodan/host/search?key=%s&query=hostname:%s", s.API.Key, domain)
}
