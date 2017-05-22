package newstojson

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	rss "github.com/jteeuwen/go-pkg-rss"
)

// News container
type NewsCustom struct {
	Title       string
	Description string
	Content     string
	Link        *url.URL
	PubTime     time.Time // Pubblication time
}

func TestParse(t *testing.T) {
	activationTime := time.Now()

	content, _ := ioutil.ReadFile("testdata/data.rss")
	feed := rss.New(1, true, chanTestHandler, func(feed *rss.Feed, ch *rss.Channel, newitems []*rss.Item) {
		log.Println("Parsing all items...")
		for _, item := range newitems {
			newitem, err := Parse(item)
			newitem.CompleteParse()
			if err != nil {
				t.Error(err)
			}
			log.Println("==================================================================================================")
			log.Println("ID           :", newitem.ID)
			log.Println("Title        : " + newitem.Title)
			log.Println("Author       : " + newitem.Author)
			log.Println("Link         :", newitem.Link)
			log.Println("Pub time     :", newitem.PubTime)
			log.Println("Mod time     :", newitem.ModTime)
			log.Println("Description  : " + newitem.Description)
			log.Println("Courses      :", newitem.Courses)
			log.Println("Degrees      :", newitem.DegreeIds)
			log.Println("#Attachments :", len(newitem.Attachments))
			log.Println("IsNew        :", newitem.IsNew(activationTime))
			PrintAttachments(newitem.Attachments)
			log.Println("Content      : " + newitem.Content)

			n := NewsCustom{
				Title:       newitem.Title,
				Description: newitem.Description,
				Content:     newitem.Content,
				Link:        newitem.Link,
				PubTime:     newitem.PubTime,
			}
			log.Println("Printing JSON...")
			res1B, _ := json.Marshal(n)
			fmt.Println(string(res1B))

		}
		log.Println("==================================================================================================")
	})
	feed.FetchBytes("http://example.com", content, nil)
}

func TestNewsPageLinksFromURLCorso(t *testing.T) {
	url := "www.dbt.univr.it/?ent=cs&tcs=N"

	correctUrls := []string{
		"www.dbt.univr.it/?ent=avvisoin&cs=385",
		"www.dbt.univr.it/?ent=avvisoin&cs=386",
		"www.dbt.univr.it/?ent=avvisoin&cs=419",
	}

	res, err := NewsPageLinksFromURLCorso(url)
	if err != nil {
		t.Error(err)
	}

	if len(res) < 1 {
		t.Error("Expected a value greater than", len(res))
	}
	if !reflect.DeepEqual(correctUrls, res) {
		t.Error("Expected", correctUrls, "got ", res)
	}

	url = "**()(&**(&*(^&*("
	_, err = NewsPageLinksFromURLCorso(url)
	if err == nil {
		t.Error("Expected error but got nil")
	}
}

func TestRetriveLast5NewsIDsFromNewsPage(t *testing.T) {
	res, err := RetriveLast5NewsIDsFromNewsPage("www.di.univr.it/?ent=avvisoin&cs=417")
	if err != nil {
		t.Error(err)
	}
	if len(res) != 5 {
		t.Error("Expected 5 elements, got", len(res))
	}
}

func TestGetNewsPagesFromHost(t *testing.T) {
	res, err := getNewsPagesFromHost("www.di.univr.it")
	if err != nil {
		t.Error(err)
	}
	if len(res) != 6 {
		t.Error("Expected 6 elements, got", len(res))
	}

	res, err = getNewsPagesFromHostMedicina("www.medicina.univr.it")
	if err != nil {
		t.Error(err)
	}
	if len(res) != 22 {
		t.Error("Expected 22 elements, got", len(res))
	}
}

func TestIsNew(t *testing.T) {

	// activationTime := time.Now()

	var tests = []struct {
		pubTime        time.Time
		modTime        time.Time
		activationTime time.Time
		res            bool // expected result
	}{
		// Date(year int, month Month, day, hour, min, sec, nsec int, loc *Location)
		{time.Date(2017, 12, 0, 12, 15, 30, 918273645, time.UTC), time.Date(2017, 12, 0, 12, 16, 30, 918273645, time.UTC), time.Date(2017, 0, 0, 12, 15, 30, 918273645, time.UTC), true},
		{time.Date(2017, 12, 0, 12, 15, 30, 918273645, time.UTC), time.Date(2018, 1, 1, 12, 14, 30, 918273645, time.UTC), time.Date(2018, 1, 1, 12, 15, 30, 918273645, time.UTC), false},
		{time.Date(2017, 12, 0, 12, 15, 30, 918273645, time.UTC), time.Date(2017, 12, 0, 12, 15, 30, 918273645, time.UTC), time.Date(2017, 12, 0, 12, 15, 30, 918273645, time.UTC), true},
	}
	for _, tt := range tests {
		news := News{}
		news.PubTime = tt.pubTime
		news.ModTime = tt.modTime
		res := news.IsNew(tt.activationTime)
		if res != tt.res {
			t.Error("Expected", tt.res, " but got", res, "\nmodTime:", tt.modTime, "\npubTime:", tt.pubTime, "\nActTime:", tt.activationTime)
		}
	}
}

func PrintAttachments(attachs []Attachment) {
	for _, item := range attachs {
		log.Println("-----------------------------------------")
		log.Println("Title   :", item.Title)
		log.Println("Link    :", item.Link)
		//log.Println("Preview :", item.Preview)
	}
	if len(attachs) > 0 {
		log.Println("-----------------------------------------")
	}
}

func chanTestHandler(feed *rss.Feed, newchannels []*rss.Channel) {
	println(len(newchannels), "new channel(s) in", feed.Url)
}

func TestParseFromLink(t *testing.T) {
	// activationTime := time.Now()

	var tests = []struct {
		url string // input
		// expected result
		id          int
		modtime     bool
		attachments int
	}{
		{"http://www.di.univr.it/?ent=avviso&dest=&id=119016&lang=eng", 119016, false, 0},
		{"http://www.di.univr.it/?dest=&ent=avviso&id=123492&lang=eng", 123492, false, 0},
		{"http://www.di.univr.it/?ent=avviso&dest=&id=118991&lang=eng", 118991, true, 0},
		{"http://www.medicina.univr.it/fol/?ent=avviso&dest=25&id=119149", 119149, true, 1},
	}
	for _, tt := range tests {
		tmp, _ := url.Parse(tt.url)
		newitem, err := ParseFromLink(tmp)
		if err != nil {
			t.Error(err)
		}
		err = newitem.SetIDsCourses()
		if err != nil {
			t.Error(err)
		}
		if newitem.ID != tt.id {
			t.Errorf("ID(%s): expected %d, actual %d", tt.url, tt.id, newitem.ID)
		}
		if strings.Compare(newitem.Author, "") == 0 {
			t.Errorf("author(%s): expected %s, actual %s", tt.url, "an atuhor", newitem.Author)
		}
		if strings.Compare(newitem.Title, "") == 0 {
			t.Errorf("title(%s): expected %s, actual %s", tt.url, "a title", newitem.Title)
		}
		if strings.Compare(newitem.Link.String(), "") == 0 {
			t.Errorf("link(%s): expected %s, actual %s", tt.url, "a valid link", newitem.Link.RawPath)
		}
		if newitem.PubTime.IsZero() {
			t.Errorf("pubtim(%s): expected %s, actual %s", tt.url, "a valid pub time", "vero")
		}
		if newitem.ModTime.IsZero() != tt.modtime {
			t.Errorf("modtime(%s): expected %t, actual %t", tt.url, tt.modtime, newitem.ModTime.IsZero())
		}
		if len(newitem.Attachments) != tt.attachments {
			t.Errorf("attachments(%s): expected %d attachments, actual %d", tt.url, tt.attachments, len(newitem.Attachments))
		}
	}
}

func TestSetIDFromURL(t *testing.T) {
	var item News
	item.SetIDFromURL("www.univr.it/?enc=avviso&id=100")
	if item.ID != 100 {
		t.Error("Expected 100 but got", item.ID)
	}

	item.SetIDFromURL("/?enc=avviso&id=101")
	if item.ID != 101 {
		t.Error("Expected 101 but got", item.ID)
	}
}
