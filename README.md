# UniVR news to JSON
This library converts news from the UniVR departments' sites to a JSON packet.

## Getting started

To parse a news from a specific URL, create the link and then pass it to `ParseFromLink` function. For example:
```
url, _ := url.Parse("http://www.di.univr.it/?ent=avviso&dest=&id=119016&lang=eng")
item, err := ParseFromLink(url)
if err != nil {
    t.Error(err)
}
...
```

This library is also compatible with an RSS item from [this](github.com/jteeuwen/go-pkg-rss) RSS feed library.
