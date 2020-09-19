// Copyright 2017 Jeff Foley. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package sources

import (
	"fmt"

	"github.com/OWASP/Amass/amass/core"
	"github.com/OWASP/Amass/amass/utils"
)

// Riddler is the Service that handles access to the Riddler data source.
type Riddler struct {
	core.BaseService

	SourceType string
}

// NewRiddler returns he object initialized, but not yet started.
func NewRiddler(config *core.Config, bus *core.EventBus) *Riddler {
	r := &Riddler{SourceType: core.SCRAPE}

	r.BaseService = *core.NewBaseService(r, "Riddler", config, bus)
	return r
}

// OnStart implements the Service interface
func (r *Riddler) OnStart() error {
	r.BaseService.OnStart()

	go r.startRootDomains()
	return nil
}

func (r *Riddler) startRootDomains() {
	// Look at each domain provided by the config
	for _, domain := range r.Config().Domains() {
		r.executeQuery(domain)
	}
}

func (r *Riddler) executeQuery(domain string) {
	url := r.getURL(domain)
	page, err := utils.RequestWebPage(url, nil, nil, "", "")
	if err != nil {
		r.Config().Log.Printf("%s: %s: %v", r.String(), url, err)
		return
	}

	r.SetActive()
	re := r.Config().DomainRegex(domain)
	for _, sd := range re.FindAllString(page, -1) {
		r.Bus().Publish(core.NewNameTopic, &core.Request{
			Name:   cleanName(sd),
			Domain: domain,
			Tag:    r.SourceType,
			Source: r.String(),
		})
	}
}

func (r *Riddler) getURL(domain string) string {
	format := "https://riddler.io/search?q=pld:%s"

	return fmt.Sprintf(format, domain)
}
