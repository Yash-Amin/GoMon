package main

import (
	"fmt"
	"golang.org/x/net/html"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Flag inputs
var crawlIncludePathRegexString = "test"
var crawlExcludePathRegexString = "^a" // match nothing
var downloadIncludePathRegexString = ".*"
var downloadExcludePathRegexString = "^a" // match nothing
var downloadExtensionsRegexString = "png"
var outputDir = "test/output"
var download = true

// Todo: status code
// Todo: mimetype

// Regex of flags
var crawlIncludePathRegex = regexp.MustCompile(crawlIncludePathRegexString)
var crawlExcludePathRegex = regexp.MustCompile(crawlExcludePathRegexString)
var downloadExtensionsRegex = regexp.MustCompile(downloadExtensionsRegexString)
var downloadIncludePathRegex = regexp.MustCompile(downloadIncludePathRegexString)
var downloadExcludePathRegex = regexp.MustCompile(downloadExcludePathRegexString)

// WaitGroups for crawler
var urlsWG sync.WaitGroup
var crawlerWG sync.WaitGroup

// To store processed URLs so we don't send requests to them again
var crawlerProcessedURLs []string

func getAbsoluteURL(base string, path string) string {
	abs, err := url.Parse(base)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	u, err := url.Parse(path)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	// fmt.Println(abs.ResolveReference(u).String())
	return abs.ResolveReference(u).String()
}

func getAttributeOfElement(node *html.Node, tag string, attribute string, values *[]string) {
	var scraper func(*html.Node)
	attribute = strings.ToLower(attribute) // ??? should convert to lowercase or not?

	scraper = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == tag {
			for _, attr := range node.Attr {
				if strings.ToLower(attr.Key) == attribute { // here too..
					*values = append(*values, attr.Val)
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			scraper(child)
		}
	}
	scraper(node)
}

func getLinks(pageURL string, res string) []string {
	doc, err := html.Parse(strings.NewReader(res))
	var values []string
	if err != nil {
		return values
	}

	getAttributeOfElement(doc, "a", "href", &values)
	// TODO: Forms, images, link, script, meta, video, audio ...
	for i, value := range values {
		values[i] = getAbsoluteURL(pageURL, value)
	}
	return values
}

// If URL is not processed yet, add it to channel
func addURL(c chan string, url string) {
	for _, processedURL := range crawlerProcessedURLs {
		if processedURL == url {
			urlsWG.Done()
			// fmt.Println("\t already done\t:", url)
			return
		}
	}
	crawlerProcessedURLs = append(crawlerProcessedURLs, url)
	c <- url
	fmt.Println("\t new link\t:", url)
}

// Returns URL path, without hostname or query string
func getURLPath(_url string) string {
	parsed, err := url.Parse(_url)
	if err != nil {
		return ""
	}
	return parsed.Path
}

// Matches crawl-regex
func shouldCrawl(url string) bool {
	urlBytes := []byte(url)
	return crawlIncludePathRegex.Match(urlBytes) &&
		!crawlExcludePathRegex.Match(urlBytes)
}

// If download flag is enabled, matches URL with download-regex
func shouldSave(path string) bool {
	if !download {
		return false
	}
	split := strings.SplitN(path, ".", -1)
	ext := split[len(split)-1]
	if len(split) == 0 {
		ext = ""
	}

	return downloadIncludePathRegex.Match([]byte(path)) &&
		downloadExtensionsRegex.Match([]byte(ext))
}

// Creates directories, and writes to file
func save(path string, data []byte) {
	path = outputDir + path
	dirPath := filepath.Dir(path)
	os.MkdirAll(dirPath, 0777)

	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		panic(err)
	}
}

func process(c chan string, url string) {
	defer urlsWG.Done()

	response, err := http.Get(url)
	if err != nil {
		return
	}
	defer response.Body.Close()

	bytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	// Check if Content-Type is html or not...
	contentType := response.Header.Get("Content-Type")
	isHTML := strings.Contains(contentType, "html")
	// Some websites do not return Content-Type header in response
	// So we have to parse every request

	body := string(bytes)

	// Get all the URLs from current page, and add to
	// channel
	var links = getLinks(url, body)
	for _, link := range links {
		if shouldSave(link) || shouldCrawl(link) {
			urlsWG.Add(1)
			go addURL(c, link)
		}
	}

	// Save current request body
	currentURLPath := getURLPath(url)
	if shouldSave(currentURLPath) {
		save(currentURLPath, bytes)
	}

	// _ = body
	_ = isHTML
}

// Crawler, takes URLs from channel
func crawler(c chan string) {
	defer crawlerWG.Done()

	for url := range c {
		fmt.Println("[+] Crawler: ", url)
		process(c, url)
	}
}

func startCrawler(url string) {
	var connections = 20

	c := make(chan string)

	for i := 0; i < connections; i++ {
		crawlerWG.Add(1)
		go crawler(c)
	}

	urlsWG.Add(1)
	go addURL(c, url)

	urlsWG.Wait()
	close(c)
	crawlerWG.Wait()
}

func main() {
	fmt.Println("--------------------------------------")
	url := "http://localhost/"
	startCrawler(url)
}
