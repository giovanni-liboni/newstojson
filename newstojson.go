package newstojson

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	rss "github.com/jteeuwen/go-pkg-rss"
	"github.com/tdewolff/minify"
	mhtml "github.com/tdewolff/minify/html"
)

// Attachment file to the news
type Attachment struct {
	Title   string
	Link    string
	Preview string
}

// News represents single news
type News struct {
	ID          int
	Title       string
	Description string
	Content     string
	Link        *url.URL
	Attachments []Attachment
	DipURL      string
	Author      string
	PubTime     time.Time // Pubblication time
	ModTime     time.Time // Modification time
	Courses     []string
	DegreeIds   []int // Lauree a cui e' rivolto l'avviso
}

// Parse funtion to parse rss item passes as argument
func Parse(rssitem *rss.Item) (*News, error) {
	var err error
	news := new(News)

	// Pubblication date
	news.PubTime, err = rssitem.ParsedPubDate()
	if err != nil {
		return nil, err
	}
	// Title
	news.Title = rssitem.Title

	// Description
	news.Description = rssitem.Description

	// Link
	news.Link, err = url.Parse(rssitem.Links[0].Href)
	if err != nil {
		return nil, err
	}

	// Retrive news ID
	m, _ := url.ParseQuery(news.Link.RawQuery)
	news.ID, _ = strconv.Atoi(m["id"][0])

	// Retrive other infos
	err = news.GetContentFromURL()
	if err != nil {
		return nil, err
	}

	// Return the new obj
	return news, nil
}

func ParseFromLink(link *url.URL, pubDate time.Time, description string, title string) (*News, error) {
	news := new(News)
	news.PubTime = pubDate
	news.Title = title
	news.Description = description
	news.Link = link

	// Retrive news ID
	m, _ := url.ParseQuery(news.Link.RawQuery)
	news.ID, _ = strconv.Atoi(m["id"][0])

	// Retrive other infos
	err := news.GetContentFromURL()
	if err != nil {
		return nil, err
	}

	return news, nil
}

func (item *News) CompleteParse() error {
	// Get all IDs courses
	err := item.SetIDsCourses()
	if err != nil {
		return err
	}
	return nil
}

func SpaceMap(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		if unicode.IsSymbol(r) {
			return -1
		}
		if strings.IndexRune(".!$", r) == 0 {
			return -1
		}
		return r
	}, str)
}

// GetContentFromURL builds content and files attached to the news
func (item *News) GetContentFromURL() error {
	// Minifier tool to delete extra whitespaces
	m := minify.New()
	m.AddFunc("text/html", mhtml.Minify)

	baseURL := "http://" + item.Link.Host

	doc, err := goquery.NewDocument(item.Link.String())

	if err != nil {
		return err
	}
	// Setto il contenuto dell'avviso
	item.Content, err = m.String("text/html", doc.Find(".sezione").Text())
	if err != nil {
		return err
	}

	action := ""
	doc.Find("#dettagliAvviso").Children().Each(func(i int, s *goquery.Selection) {
		if action == "pubDate" && s.Is("dd") {
			value := SpaceMap(strings.TrimSpace(s.Text()))
			layout := "Monday,January2,2006-15:4:5PM"
			item.PubTime, _ = time.Parse(layout, value)
			action = ""
		} else if action == "modDate" && s.Is("dd") {
			value := SpaceMap(strings.TrimSpace(s.Text()))
			layout := "Monday,January2,2006-15:4:5PM"
			item.ModTime, _ = time.Parse(layout, value)
			action = ""
		} else if action == "author" && s.Is("dd") {
			item.Author = strings.TrimSpace(s.Text())
			action = "courses"
		} else if action == "courses" && s.Is("dd") {
			item.Courses = append(item.Courses, strings.TrimSpace(s.Text()))
		} else {
			action = ""
		}

		current := strings.TrimSpace(s.Text())
		if current == "Publication date" {
			action = "pubDate"
		} else if current == "Last Modified" {
			action = "modDate"
		} else if current == "Published by" {
			action = "author"
		}
	})

	// Searching for Attachments
	attach := Attachment{}
	doc.Find(".formati").Find("li").Each(func(i int, s *goquery.Selection) {
		attach.Title, err = m.String("text/html", s.Text())
		if err != nil {
			log.Println("Error on title : " + err.Error())
		}
		linkAllegato, isPresent := s.Find("a").Attr("href")

		if isPresent {
			attach.Link = baseURL + linkAllegato

			onclickString, isPresent := s.Find("a").Attr("onclick")
			if isPresent {
				attach.Link, err = m.String("text/html", baseURL+strings.Split(onclickString, "'")[3])
				if err != nil {
					log.Println("Error : " + err.Error())
				}

				resp, err := http.Get(attach.Link)
				if err != nil {
					log.Println("Error on response : " + err.Error())
				} else {
					defer resp.Body.Close()
					body, err := ioutil.ReadAll(resp.Body)
					attach.Preview, err = m.String("text/html", string(body))
					if err != nil {
						log.Println("Error on retrived : " + err.Error())
					}
				}
			}
		}
		item.Attachments = append(item.Attachments, attach)
	})
	// End Searching for Attachments

	return nil
}

func (item *News) SetIDsCourses() error {

	// Get all news pages
	newsPageList, err := getNewsPagesFromHost(item.Link.Host)
	if err != nil {
		return err
	}
	for _, val := range newsPageList {
		//Recupero gli ultimi 5 avvisi da ogni corso e vedo dove e' presente
		ids, err := RetriveLast5NewsIDsFromNewsPage(val)
		if err != nil {
			log.Println(err)
			return err
		}
		if contains(ids, item.ID) {
			item.DegreeIds = append(item.DegreeIds, GetIDFromCompleteURL(val))
		}
	}

	return nil
}

func getNewsPagesFromHost(host string) ([]string, error) {
	var res []string

	// Triennali
	tmpRes, err := NewsPageLinksFromURLCorso(host + "/?ent=cs&tcs=N")
	if err != nil {
		return nil, err
	}

	res = append(res, tmpRes...)
	// Magistrali
	tmpRes, err = NewsPageLinksFromURLCorso(host + "/?ent=cs&tcs=MA")
	if err != nil {
		return nil, err
	}
	res = append(res, tmpRes...)

	return res, nil
}

// Ritorna la lista dei link alle pagine che contengono gli avvisi del dipartimento passato come parametro a partire dall'url che contiene tutte le laure del corso
func NewsPageLinksFromURLCorso(urlString string) ([]string, error) {
	var res []string

	urlString = "http://" + urlString

	rootURL, err := url.Parse(urlString)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	doc, err := goquery.NewDocument(urlString)

	if err != nil {
		log.Println(err)
		return nil, err
	} else {
		doc.Find("#centroservizi").Find("dl").Find("dt").Find("a").Each(func(i int, s *goquery.Selection) {

			tokens := strings.Split(s.Text(), "(")
			var stringToTest string

			if len(tokens) > 1 {
				stringToTest = tokens[1]
			} else {
				stringToTest = tokens[0]
			}
			// Elimino tutti i corsi che non sono piu' validi
			if !strings.Contains(stringToTest, "until") {
				// Recupero l'ID
				idString, idBool := s.Attr("href")
				if idBool {
					id := GetIDFromString(idString)

					//log.Println(rootURL.Host + "/?ent=avvisoin&cs=" + strconv.Itoa(id))
					res = append(res, rootURL.Host+"/?ent=avvisoin&cs="+strconv.Itoa(id))
				}
			}
		})
	}
	return res, nil
}
func RetriveLast5NewsIDsFromNewsPage(newsPageURL string) ([]int, error) {
	var res []int

	newsPageURL = "http://" + newsPageURL

	doc, err := goquery.NewDocument(newsPageURL)
	if err != nil {
		return nil, err
	} else {
		if doc.Find("table").Find("tbody").Find("tr").Find("a").Size() > 5 {
			doc.Find("table").Find("tbody").Find("tr").Find("a").Slice(0, 5).Each(func(i int, s *goquery.Selection) {
				idString, idBool := s.Attr("href")
				if idBool {
					id := GetIDFromString(idString)
					// log.Println(rootURL.Host + "/?ent=avvisoin&cs=" + strconv.Itoa(id))
					res = append(res, id)
				}
			})
		} else {
			doc.Find("table").Find("tbody").Find("tr").Find("a").Each(func(i int, s *goquery.Selection) {
				idString, idBool := s.Attr("href")
				if idBool {
					id := GetIDFromString(idString)
					// log.Println(rootURL.Host + "/?ent=avvisoin&cs=" + strconv.Itoa(id))
					res = append(res, id)
				}
			})
		}
	}

	return res, nil
}

// GetIDFromString retrive int ID from a string like this /?ent=avvisoin&id=432
func GetIDFromString(urlString string) int {
	var id int
	// Costruisco l'url completo alla pagina
	singleURL, _ := url.Parse("www.example.com" + urlString)

	// recupero l'ID del corso
	m, _ := url.ParseQuery(singleURL.RawQuery)
	if val, ok := m["id"]; ok {
		id, _ = strconv.Atoi(val[0])
	} else if val, ok := m["cs"]; ok {
		id, _ = strconv.Atoi(val[0])
	} else {
		log.Println(urlString)
		id = 0
	}

	return id
}

// GetIDFromString retrive int ID from a string like this www.univr.it/?ent=avvisoin&id=432
func GetIDFromCompleteURL(urlString string) int {
	var id int
	// Costruisco l'url completo alla pagina
	singleURL, _ := url.Parse(urlString)

	// recupero l'ID del corso
	m, _ := url.ParseQuery(singleURL.RawQuery)
	if val, ok := m["id"]; ok {
		id, _ = strconv.Atoi(val[0])
	} else if val, ok := m["cs"]; ok {
		id, _ = strconv.Atoi(val[0])
	} else {
		log.Println(urlString)
		id = 0
	}

	return id
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
