package newstojson

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	log "github.com/Sirupsen/logrus"
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

// =============================================================================
// Parse functions
// =============================================================================

// Parse function to parse rss item passes as argument
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
	news.ID, _ = strconv.Atoi(m.Get("id"))

	// Retrive other infos
	err = news.GetContentFromURL()
	if err != nil {
		return nil, err
	}

	// Return the new obj
	return news, nil
}

// ParseFromLink parse a new from a direct link
func ParseFromLink(link *url.URL) (*News, error) {
	news := new(News)
	news.Link = link

	// Retrive news ID
	m, _ := url.ParseQuery(news.Link.RawQuery)
	news.ID, _ = strconv.Atoi(m.Get("id"))

	// Retrive other infos
	err := news.GetContentFromURL()
	if err != nil {
		return nil, err
	}

	return news, nil
}

// CompleteParse retrive all the informations
func (item *News) CompleteParse() error {
	// Get all IDs courses
	err := item.SetIDsCourses()
	if err != nil {
		return err
	}
	return nil
}

// =============================================================================
// Secondary functions (IsNew)
// =============================================================================

// IsNew Return true if the news was sent BEFORE activation date
func (item *News) IsNew(activationTime time.Time) bool {
	return activationTime.UTC().Before(item.PubTime.UTC()) || activationTime.UTC().Equal(item.PubTime.UTC()) || activationTime.UTC().Before(item.ModTime.UTC()) || activationTime.UTC().Equal(item.ModTime.UTC())
}

// GetContentFromURL builds content and files attached to the news
func (item *News) GetContentFromURL() error {
	// Minifier tool to delete extra whitespaces
	m := minify.New()
	m.AddFunc("text/html", mhtml.Minify)

	link := item.Link

	l, _ := url.ParseQuery(item.Link.RawQuery)
	if len(l["lang"]) > 1 {
		if strings.Compare(l.Get("lang"), "eng") != 0 {
			l.Del("lang")
			l.Set("lang", "eng")
		}
	} else {
		l.Set("lang", "eng")
	}
	link.RawQuery = l.Encode()

	baseURL := "http://" + item.Link.Host

	doc, err := goquery.NewDocument(link.String())

	if err != nil {
		return err
	}
	// Setto il contenuto dell'avviso
	if doc.Find(".main-text").Text() != "" {
		item.Content, err = m.String("text/html", doc.Find(".main-text").Text())
		if err != nil {
			return err
		}
	} else {
		item.Content, err = m.String("text/html", doc.Find(".sezione").Text())
		if err != nil {
			return err
		}
	}
	item.Title = doc.Find("h1").Text()

	action := ""
	doc.Find("#dettagliAvviso").Children().Each(func(i int, s *goquery.Selection) {
		if action == "pubDate" && s.Is("dd") {
			value := SpaceMap(strings.TrimSpace(s.Text()))
			layout := "Monday,January2,2006-15:4:5PM"
			loc, _ := time.LoadLocation("Europe/Rome")
			item.PubTime, _ = time.ParseInLocation(layout, value, loc)
			action = ""
		} else if action == "modDate" && s.Is("dd") {
			value := SpaceMap(strings.TrimSpace(s.Text()))
			layout := "Monday,January2,2006-15:4:5PM"
			loc, _ := time.LoadLocation("Europe/Rome")
			item.ModTime, _ = time.ParseInLocation(layout, value, loc)
			action = ""
		} else if action == "author" && s.Is("dd") {
			html, errIn := s.Html()
			if err != nil {
				log.Errorln(errIn)
			}
			res := HtmlBRDivisorTOArray(html)
			item.Author = res[0]
			item.Courses = res[1:]
			for i, course := range item.Courses {
				item.Courses[i], err = m.String("text/html", course)
				if err != nil {
					log.Errorln(err)
				}
			}

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
			log.Errorln(err)
		}
		attach.Title = removeExtraSpaces(attach.Title)

		linkAllegato, isPresent := s.Find("a").Attr("href")

		if isPresent {
			attach.Link = baseURL + linkAllegato

			onclickString, isPresent := s.Find("a").Attr("onclick")
			if isPresent {
				attach.Link, err = m.String("text/html", baseURL+strings.Split(onclickString, "'")[3])
				if err != nil {
					log.Errorln(err)
				}

				resp, err := http.Get(attach.Link)
				if err != nil {
					log.Errorln("Error on response : " + err.Error())
				} else {
					defer resp.Body.Close()
					body, _ := ioutil.ReadAll(resp.Body)
					attach.Preview, err = m.String("text/html", string(body))
					if err != nil {
						log.Errorln(err)
					}
				}
			}
		}
		item.Attachments = append(item.Attachments, attach)
	})
	// End Searching for Attachments

	return nil
}

// SetIDsCourses sets id courses
func (item *News) SetIDsCourses() error {
	var newsPageList []string
	var err error
	// Get all news pages
	if strings.Contains(item.Link.Host, "medicina") {
		newsPageList, err = getNewsPagesFromHostMedicina(item.Link.Host)
	} else {
		newsPageList, err = getNewsPagesFromHost(item.Link.Host)
	}
	if err != nil {
		return err
	}
	for _, val := range newsPageList {
		//Recupero gli ultimi 5 avvisi da ogni corso e vedo dove e' presente
		ids, err := RetriveLast5NewsIDsFromNewsPage(val)
		if err != nil {
			return err
		}
		if contains(ids, item.ID) {
			item.DegreeIds = append(item.DegreeIds, getIDFromCompleteURL(val))
		}
	}

	return nil
}

// SetIDFromURL set the news's ID from the direct link
func (item *News) SetIDFromURL(url string) error {
	if strings.Contains(url, "avviso") {
		if strings.Contains(url, "univr.it") {
			// host gia' presente
			item.ID = getIDFromCompleteURL(url)
		} else {
			item.ID = getIDFromCompleteURL("wwww.example.com" + url)
		}
	} else {
		return errors.New("No valid url")
	}
	return nil
}

// =============================================================================
// HTML utils functions
// =============================================================================

func getNewsPagesFromHost(host string) ([]string, error) {
	var res []string
	coursesType := []string{
		"N",
		"MA",
		"mu",
		"SP",
		"R",
		"F",
		"T",
	}
	for _, courseType := range coursesType {
		tmpRes, err := NewsPageLinksFromURLCorso(host + "/?ent=cs&tcs=" + courseType)
		if err != nil {
			return nil, err
		}
		res = append(res, tmpRes...)
	}
	return res, nil
}

// getNewsPagesFromHostMedicina recupera i link delle pagine
func getNewsPagesFromHostMedicina(host string) ([]string, error) {
	var res []string
	coursesType := []string{
		"N",
		"MA",
		"mu",
		"SP",
		"R",
		"F",
		"T",
	}
	for _, courseType := range coursesType {
		tmpRes, err := NewsPageLinksFromURLCorsoMedicina(host + "/?ent=cs&tcs=" + courseType)
		if err != nil {
			return nil, err
		}
		res = append(res, tmpRes...)
	}
	return res, nil
}

// NewsPageLinksFromURLCorso Ritorna la lista dei link alle pagine che contengono gli avvisi del
// dipartimento passato come parametro a partire dall'url che contiene
// tutte le laure del corso
func NewsPageLinksFromURLCorso(urlString string) ([]string, error) {
	var res []string

	urlString = "http://" + urlString

	rootURL, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocument(urlString)

	if err != nil {
		return nil, err
	}
	doc.Find("#contenutoPagina").Find("div").First().Find("dl").Find("dt").Find("a").Each(func(i int, s *goquery.Selection) {

		// Costrisco l'intero url
		idString, idBool := s.Attr("href")
		if idBool {
			id := getIDFromString(idString)
			// log.Println(rootURL.Host + "/?ent=avvisoin&cs=" + strconv.Itoa(id))
			res = append(res, rootURL.Host+"/?ent=avvisoin&cs="+strconv.Itoa(id))
		}
	})

	return res, nil
}

// NewsPageLinksFromURLCorsoMedicina retrive information from a url based on "medicina" url.
func NewsPageLinksFromURLCorsoMedicina(urlString string) ([]string, error) {
	var res []string

	urlString = "http://" + urlString

	rootURL, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocument(urlString)

	if err != nil {
		return nil, err
	}
	doc.Find("#centroservizi").Find("dl").Find("dt").Find("a").Each(func(i int, s *goquery.Selection) {
		// Start OLD SITE
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
				id := getIDFromString(idString)

				//log.Println(rootURL.Host + "/?ent=avvisoin&cs=" + strconv.Itoa(id))
				res = append(res, rootURL.Host+"/?ent=avvisoin&cs="+strconv.Itoa(id))
			}
		}
		// END OLD SITE
	})

	return res, nil
}

// RetriveLast5NewsIDsFromNewsPage retrives last 5 news ids from a news page
func RetriveLast5NewsIDsFromNewsPage(newsPageURL string) ([]int, error) {
	var res []int

	newsPageURL = "http://" + newsPageURL

	doc, err := goquery.NewDocument(newsPageURL)
	if err != nil {
		return nil, err
	}
	if doc.Find("table").Find("tbody").Find("tr").Find("a").Size() > 5 {
		doc.Find("table").Find("tbody").Find("tr").Find("a").Slice(0, 5).Each(func(i int, s *goquery.Selection) {
			idString, idBool := s.Attr("href")
			if idBool {
				id := getIDFromString(idString)
				// log.Println(rootURL.Host + "/?ent=avvisoin&cs=" + strconv.Itoa(id))
				res = append(res, id)
			}
		})
	} else {
		doc.Find("table").Find("tbody").Find("tr").Find("a").Each(func(i int, s *goquery.Selection) {
			idString, idBool := s.Attr("href")
			if idBool {
				id := getIDFromString(idString)
				// log.Println(rootURL.Host + "/?ent=avvisoin&cs=" + strconv.Itoa(id))
				res = append(res, id)
			}
		})
	}

	return res, nil
}

// GetIDFromString retrive int ID from a string like this /?ent=avvisoin&id=432
func getIDFromString(urlString string) int {
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
		log.Warnln(urlString)
		id = 0
	}

	return id
}

// GetIDFromString retrive int ID from a string like this www.univr.it/?ent=avvisoin&id=432
func getIDFromCompleteURL(urlString string) int {
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
		log.Warnln(urlString)
		id = 0
	}

	return id
}

// =============================================================================
// Utils functions
// =============================================================================

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// HtmlBRDivisorTOArray splits HTML text inside <br> into an array
func HtmlBRDivisorTOArray(html string) (res []string) {
	res = strings.Split(html, "<br/>")
	for i, tmp := range res {
		res[i] = removeExtraSpaces(tmp)
	}
	return res
}

func removeExtraSpaces(s string) string {
	ReLeadCloseWhtsp := regexp.MustCompile(`^[\s\p{Zs}]+|[\s\p{Zs}]+$`)
	ReInsideWhtsp := regexp.MustCompile(`[\s\p{Zs}]{2,}`)
	final := ReLeadCloseWhtsp.ReplaceAllString(s, "")
	final = ReInsideWhtsp.ReplaceAllString(final, " ")
	return final
}

// SpaceMap remove unwanted symbol inside a string
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
