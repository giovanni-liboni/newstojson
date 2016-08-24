package newstojson

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"reflect"
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
			PrintAttachments(newitem.Attachments)

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
		"www.dbt.univr.it/?ent=avvisoin&cs=386",
		"www.dbt.univr.it/?ent=avvisoin&cs=385",
		"www.dbt.univr.it/?ent=avvisoin&cs=419",
	}

	res, _ := NewsPageLinksFromURLCorso(url)

	if len(res) < 1 {
		t.Error("Expected a value greater than", len(res))
	}
	if !reflect.DeepEqual(correctUrls, res) {
		t.Error("Expected", correctUrls, "got ", res)
	}
}
func TestRetriveLast5NewsIDsFromNewsPage(t *testing.T) {
	res, err := RetriveLast5NewsIDsFromNewsPage("www.dbt.univr.it/?ent=avvisoin&cs=385")
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
	if len(res) != 7 {
		t.Error("Expected 7 elements, got", len(res))
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
