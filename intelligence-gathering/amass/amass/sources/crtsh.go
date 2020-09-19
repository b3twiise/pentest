// Copyright 2017 Jeff Foley. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package sources

import (
	"encoding/json"

	"github.com/OWASP/Amass/amass/core"
	"github.com/OWASP/Amass/amass/utils"
)

// Crtsh is the Service that handles access to the Crtsh data source.
type Crtsh struct {
	core.BaseService

	SourceType string
}

// NewCrtsh returns he object initialized, but not yet started.
func NewCrtsh(config *core.Config, bus *core.EventBus) *Crtsh {
	c := &Crtsh{SourceType: core.CERT}

	c.BaseService = *core.NewBaseService(c, "Crtsh", config, bus)
	return c
}

// OnStart implements the Service interface
func (c *Crtsh) OnStart() error {
	c.BaseService.OnStart()

	go c.startRootDomains()
	return nil
}

func (c *Crtsh) startRootDomains() {
	// Look at each domain provided by the config
	for _, domain := range c.Config().Domains() {
		c.executeQuery(domain)
	}
}

func (c *Crtsh) executeQuery(domain string) {
	url := c.getURL(domain)
	page, err := utils.RequestWebPage(url, nil, nil, "", "")
	if err != nil {
		c.Config().Log.Printf("%s: %s: %v", c.String(), url, err)
		return
	}

	c.SetActive()
	// Extract the subdomain names from the results
	var results []struct {
		Name string `json:"name_value"`
	}
	if err := json.Unmarshal([]byte(page), &results); err != nil {
		return
	}
	for _, line := range results {
		c.Bus().Publish(core.NewNameTopic, &core.Request{
			Name:   line.Name,
			Domain: domain,
			Tag:    c.SourceType,
			Source: c.String(),
		})
	}
}

func (c *Crtsh) getURL(domain string) string {
	return "https://crt.sh/?q=%25." + domain + "&output=json"
}
