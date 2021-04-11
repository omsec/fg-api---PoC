package client

import (
	"sync"
	"time"
)

type request struct {
	ProfileID string
	Accessed  time.Time
}

// https://blog.golang.org/maps
// mediate access to the requests-map using mutex
// this is needed because the map is maintained by a GO-routine
var registry = struct {
	sync.RWMutex
	requests map[string]request // key is IP or domain-action (eg. course-search)
}{}

type Registry struct {
}

func (r Registry) Initialize() {
	//requests = make(map[string]request)
	registry.requests = make(map[string]request)
}

func (r Registry) Continue(client string, profileID string) bool {

	// ToDo:
	// vielleicht das risiko eingehen und hier ohne lock zugreifen

	// combination of client & url found = this was a page refresh
	registry.RLock()
	found := !(registry.requests[client].ProfileID == profileID)
	registry.RUnlock()

	// add or update the last (relevant) request
	req := request{
		ProfileID: profileID,
		Accessed:  time.Now(),
	}

	registry.Lock()
	registry.requests[client] = req
	registry.Unlock()

	return found
}

// Fush removes requests from the registry which are older than 15 minutes
// usually called by a GO-routine that runs in a ticker
func (r Registry) Flush() {

	registry.Lock()
	now := time.Now()
	if len(registry.requests) > 5000 {
		// it's safe to just delete expired keys, since iterations over maps are not ordered
		for key, value := range registry.requests {
			// remove if last access was 15 mins ago
			if now.Sub(value.Accessed).Minutes() > 15 {
				delete(registry.requests, key)
			}
		}
	}
	registry.Unlock()
}

// Count returns how many different client are currently active
func (r Registry) Count() int {
	registry.RLock()
	cnt := len(registry.requests)
	registry.RUnlock()
	return cnt
}

// Dump returns the last accessed profile and timestamp for each client
func (r Registry) Dump(max int) []request {

	var res []request
	var req request
	i := 0
	for _, v := range registry.requests {
		if i > max {
			break
		}

		req.ProfileID = v.ProfileID
		req.Accessed = v.Accessed

		res = append(res, req)
	}

	return res
}
