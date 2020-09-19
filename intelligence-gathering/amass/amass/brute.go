// Copyright 2017 Jeff Foley. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package amass

import (
	"strings"
	"sync"
	"time"

	"github.com/OWASP/Amass/amass/core"
	"github.com/OWASP/Amass/amass/utils"
	"github.com/miekg/dns"
)

// BruteForceQueryTypes contains the DNS record types that service queries for.
var BruteForceQueryTypes = []string{
	"CNAME",
	"A",
	"AAAA",
}

// BruteForceService is the Service that handles all brute force name generation
// within the architecture.
type BruteForceService struct {
	core.BaseService

	metrics    *core.MetricsCollector
	totalLock  sync.RWMutex
	totalNames int

	max    utils.Semaphore
	filter *utils.StringFilter
}

// NewBruteForceService returns he object initialized, but not yet started.
func NewBruteForceService(config *core.Config, bus *core.EventBus) *BruteForceService {
	bfs := &BruteForceService{
		max:    utils.NewSimpleSemaphore(5000),
		filter: utils.NewStringFilter(),
	}

	bfs.BaseService = *core.NewBaseService(bfs, "Brute Forcing", config, bus)
	return bfs
}

// OnStart implements the Service interface.
func (bfs *BruteForceService) OnStart() error {
	bfs.BaseService.OnStart()

	bfs.metrics = core.NewMetricsCollector(bfs)
	bfs.metrics.NamesRemainingCallback(bfs.namesRemaining)

	if bfs.Config().BruteForcing {
		if bfs.Config().Recursive {
			if bfs.Config().MinForRecursive == 0 {
				bfs.Bus().Subscribe(core.NameResolvedTopic, bfs.SendRequest)
				go bfs.processRequests()
			} else {
				bfs.Bus().Subscribe(core.NewSubdomainTopic, bfs.NewSubdomain)
			}
		}
		go bfs.startRootDomains()
	}
	return nil
}

// OnStop implements the Service interface.
func (bfs *BruteForceService) OnStop() error {
	bfs.metrics.Stop()
	return nil
}

func (bfs *BruteForceService) processRequests() {
	for {
		select {
		case <-bfs.PauseChan():
			<-bfs.ResumeChan()
		case <-bfs.Quit():
			return
		case req := <-bfs.RequestChan():
			if bfs.goodRequest(req) {
				go bfs.performBruteForcing(req.Name, req.Domain)
			}
		}
	}
}

func (bfs *BruteForceService) goodRequest(req *core.Request) bool {
	var ok bool
	if !bfs.Config().IsDomainInScope(req.Name) {
		return ok
	}

	bfs.SetActive()
	for _, r := range req.Records {
		t := uint16(r.Type)

		if t == dns.TypeA || t == dns.TypeAAAA {
			ok = true
			break
		}
	}
	return ok
}

func (bfs *BruteForceService) startRootDomains() {
	// Look at each domain provided by the config
	for _, domain := range bfs.Config().Domains() {
		go bfs.performBruteForcing(domain, domain)
	}
}

// NewSubdomain is called by the Name Service when proper subdomains are discovered.
func (bfs *BruteForceService) NewSubdomain(req *core.Request, times int) {
	if times == bfs.Config().MinForRecursive {
		go bfs.performBruteForcing(req.Name, req.Domain)
	}
}

func (bfs *BruteForceService) performBruteForcing(subdomain, domain string) {
	subdomain = strings.ToLower(subdomain)
	domain = strings.ToLower(domain)
	req := &core.Request{
		Name:   subdomain,
		Domain: domain,
	}
	if subdomain == "" || domain == "" || bfs.filter.Duplicate(subdomain) ||
		GetWildcardType(req) == WildcardTypeDynamic {
		return
	}

	bfs.totalLock.Lock()
	bfs.totalNames += len(bfs.Config().Wordlist)
	bfs.totalLock.Unlock()

	var idx int
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-bfs.Quit():
			return
		case <-t.C:
			bfs.SetActive()
		default:
			if idx >= len(bfs.Config().Wordlist) {
				return
			}
			bfs.max.Acquire(1)
			word := strings.ToLower(bfs.Config().Wordlist[idx])
			go bfs.bruteForceResolution(word, subdomain, domain)
			idx++
		}
	}
}

func (bfs *BruteForceService) bruteForceResolution(word, sub, domain string) {
	defer bfs.SetActive()
	defer bfs.max.Release(1)
	defer bfs.decTotalNames()

	if word == "" || sub == "" || domain == "" {
		return
	}

	name := word + "." + sub
	var answers []core.DNSAnswer
	for _, t := range BruteForceQueryTypes {
		if a, err := Resolve(name, t); err == nil {
			answers = append(answers, a...)
			// Do not continue if a CNAME was discovered
			if t == "CNAME" {
				bfs.metrics.QueryTime(time.Now())
				break
			}
		}
		bfs.metrics.QueryTime(time.Now())
		bfs.SetActive()
	}

	req := &core.Request{
		Name:    name,
		Domain:  domain,
		Records: answers,
		Tag:     core.BRUTE,
		Source:  bfs.String(),
	}

	bfs.SetActive()
	if len(answers) == 0 || MatchesWildcard(req) {
		return
	}
	bfs.Bus().Publish(core.NameResolvedTopic, req)
}

// Stats implements the Service interface.
func (bfs *BruteForceService) Stats() *core.ServiceStats {
	return bfs.metrics.Stats()
}

func (bfs *BruteForceService) namesRemaining() int {
	bfs.totalLock.RLock()
	defer bfs.totalLock.RUnlock()

	return bfs.totalNames
}

func (bfs *BruteForceService) decTotalNames() {
	bfs.totalLock.Lock()
	defer bfs.totalLock.Unlock()

	bfs.totalNames--
}
