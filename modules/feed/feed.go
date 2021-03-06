// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public
// License along with this program. If not, see <http://www.gnu.org/licenses/>.

package feed

import (
	"github.com/nmeum/go-feedparser"
	"github.com/nmeum/marvin/irc"
	"github.com/nmeum/marvin/modules"
	"html"
	"net/http"
	"strings"
	"sync"
	"time"
)

type post struct {
	Feed feedparser.Feed
	Item feedparser.Item
}

type Module struct {
	feeds    map[string]time.Time
	URLs     []string `json:"urls"`
	Interval string   `json:"interval"`
}

func Init(moduleSet *modules.ModuleSet) {
	moduleSet.Register(new(Module))
}

func (m *Module) Name() string {
	return "feed"
}

func (m *Module) Help() string {
	return "Display new entries for RSS/ATOM feeds."
}

func (m *Module) Defaults() {
	m.Interval = "0h15m"
}

func (m *Module) Load(client *irc.Client) error {
	m.feeds = make(map[string]time.Time)
	for _, url := range m.URLs {
		m.feeds[url] = time.Now()
	}

	duration, err := time.ParseDuration(m.Interval)
	if err != nil {
		return err
	}

	newPosts := make(chan post)
	go func() {
		for post := range newPosts {
			m.notify(client, post)
		}
	}()

	go func() {
		for {
			time.Sleep(duration)
			m.pollFeeds(newPosts)
		}
	}()

	return nil
}

func (m *Module) pollFeeds(out chan post) {
	var wg sync.WaitGroup
	for _, url := range m.URLs {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			feed, err := m.fetchFeed(u)
			if err != nil {
				return
			}

			latest := m.feeds[u]
			for _, i := range feed.Items {
				if !i.PubDate.After(latest) {
					break
				}

				out <- post{feed, i}
			}

			m.feeds[u] = feed.Items[0].PubDate
		}(url)
	}

	wg.Wait()
}

func (m *Module) fetchFeed(url string) (feed feedparser.Feed, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}

	reader := resp.Body
	defer reader.Close()

	feed, err = feedparser.Parse(reader)
	if err != nil {
		return
	}

	return
}

func (m *Module) notify(client *irc.Client, post post) {
	ftitle := html.UnescapeString(post.Feed.Title)
	for _, ch := range client.Channels {
		ititle := html.UnescapeString(post.Item.Title)
		client.Write("NOTICE %s :%s -- %s new entry %s: %s",
			ch, strings.ToUpper(m.Name()), ftitle, ititle, post.Item.Link)
	}
}
