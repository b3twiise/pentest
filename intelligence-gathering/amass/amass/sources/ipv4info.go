// Copyright 2017 Jeff Foley. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package sources

import (
	"fmt"
	"regexp"
	"time"

	"github.com/OWASP/Amass/amass/core"
	"github.com/OWASP/Amass/amass/utils"
)

// IPv4Info is the Service that handles access to the IPv4Info data source.
type IPv4Info struct {
	core.BaseService

	baseURL    string
	SourceType string
}

// NewIPv4Info returns he object initialized, but not yet started.
func NewIPv4Info(config *core.Config, bus *core.EventBus) *IPv4Info {
	i := &IPv4Info{
		baseURL:    "http://ipv4info.com",
		SourceType: core.SCRAPE,
	}

	i.BaseService = *core.NewBaseService(i, "IPv4Info", config, bus)
	return i
}

// OnStart implements the Service interface
func (i *IPv4Info) OnStart() error {
	i.BaseService.OnStart()

	go i.startRootDomains()
	return nil
}

func (i *IPv4Info) startRootDomains() {
	// Look at each domain provided by the config
	for _, domain := range i.Config().Domains() {
		i.executeQuery(domain)
	}
}

func (i *IPv4Info) executeQuery(domain string) {
	url := i.getURL(domain)
	page, err := utils.RequestWebPage(url, nil, nil, "", "")
	if err != nil {
		i.Config().Log.Printf("%s: %s: %v", i.String(), url, err)
		return
	}

	i.SetActive()
	time.Sleep(time.Second)
	url = i.ipSubmatch(page, domain)
	page, err = utils.RequestWebPage(url, nil, nil, "", "")
	if err != nil {
		i.Config().Log.Printf("%s: %s: %v", i.String(), url, err)
		return
	}

	i.SetActive()
	time.Sleep(time.Second)
	url = i.domainSubmatch(page, domain)
	page, err = utils.RequestWebPage(url, nil, nil, "", "")
	if err != nil {
		i.Config().Log.Printf("%s: %s: %v", i.String(), url, err)
		return
	}

	i.SetActive()
	time.Sleep(time.Second)
	url = i.subdomainSubmatch(page, domain)
	page, err = utils.RequestWebPage(url, nil, nil, "", "")
	if err != nil {
		i.Config().Log.Printf("%s: %s: %v", i.String(), url, err)
		return
	}

	re := i.Config().DomainRegex(domain)
	for _, sd := range re.FindAllString(page, -1) {
		i.Bus().Publish(core.NewNameTopic, &core.Request{
			Name:   cleanName(sd),
			Domain: domain,
			Tag:    i.SourceType,
			Source: i.String(),
		})
	}
}

func (i *IPv4Info) ipSubmatch(content, domain string) string {
	re := regexp.MustCompile("/ip-address/(.*)/" + domain)
	subs := re.FindAllString(content, -1)
	if len(subs) == 0 {
		return ""
	}
	return i.baseURL + subs[0]
}

func (i *IPv4Info) domainSubmatch(content, domain string) string {
	re := regexp.MustCompile("/dns/(.*?)/" + domain)
	subs := re.FindAllString(content, -1)
	if len(subs) == 0 {
		return ""
	}
	return i.baseURL + subs[0]
}

func (i *IPv4Info) subdomainSubmatch(content, domain string) string {
	re := regexp.MustCompile("/subdomains/(.*?)/" + domain)
	subs := re.FindAllString(content, -1)
	if len(subs) == 0 {
		return ""
	}
	return i.baseURL + subs[0]
}

func (i *IPv4Info) getURL(domain string) string {
	format := i.baseURL + "/search/%s"

	return fmt.Sprintf(format, domain)
}
