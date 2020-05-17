package main

import (
	"flag"
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
type inputURLArgs []string

func (values *inputURLArgs) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func (values inputURLArgs) String() string {
	return strings.Join(values, ",")
}

var crawlIncludePathRegexString = ".*"
var crawlExcludePathRegexString = "^a" // match nothing
var downloadIncludePathRegexString = ".*"
var downloadExcludePathRegexString = "^a" // match nothing
var downloadExtensionsRegexString = "."
var outputDir = "test/output"
var download = true
var startURLs inputURLArgs

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
	if node == nil {
		return
	}

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
	getAttributeOfElement(doc, "img", "src", &values)
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

// Returns URL's host
func getURLHost(_url string) string {
	parsed, err := url.Parse(_url)
	if err != nil {
		return ""
	}
	return parsed.Host
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

	return (downloadIncludePathRegex.Match([]byte(path)) &&
		downloadExtensionsRegex.Match([]byte(ext)))
}

// Creates directories, and writes to file.
//
// Error!
// Some URLs like "http://example.com/dir/" and "http://example.com/dir/file.html"
// will cause error, because in 2nd URL path is "dir/file.html" so output path
// of that URL will be "outputDir/dir/file.html" but for first URL, the directory
// dir already exists and we can not create file with the same name! so we need to
// change the name.
func save(path string, data []byte) {
	path = outputDir + "/" + path
	path = strings.ReplaceAll(path, "//", "/")
	if strings.HasSuffix(path, "/") {
		path += "__index__" // add random string
	}
	dirPath := filepath.Dir(path)
	os.MkdirAll(dirPath, 0777)
	f, err := os.Create(path)
	if err != nil {
		fmt.Println("ERROR:", path)
		// panic(err)
		return // TODO: Fix this
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
	defer func() {
		if response != nil {
			response.Body.Close()
		}
	}()
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
	currentURLHost := getURLHost(url)
	if shouldSave(currentURLPath) {
		save(currentURLHost+"/"+currentURLPath, bytes)
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

func startCrawler(urls []string) {
	var connections = 20

	c := make(chan string)

	for i := 0; i < connections; i++ {
		crawlerWG.Add(1)
		go crawler(c)
	}

	for _, url := range urls {
		urlsWG.Add(1)
		go addURL(c, url)
	}

	urlsWG.Wait()
	close(c)
	crawlerWG.Wait()
}

func flagParse() {

	var crawlIncludePathRegexString = ".*"
	var crawlExcludePathRegexString = "^a" // match nothing
	var downloadIncludePathRegexString = ".*"
	var downloadExcludePathRegexString = "^a" // match nothing
	var downloadExtensionsRegexString = "png"
	var outputDir = "test/output"
	var download = true
	flag.Var(&startURLs, "u", "Specify the URL(s) for cralwer")
	flag.StringVar(&crawlIncludePathRegexString, "crawlInclude", ".*", "Regex to include URLs for crawler")
	flag.StringVar(&crawlExcludePathRegexString, "crawlExclude", "^a", "Regex to exclude URLs for crawler")
	flag.StringVar(&downloadIncludePathRegexString, "downloadInclude", ".*", "Regex to include URLs for downloader")
	flag.StringVar(&downloadExcludePathRegexString, "downloadExclude", "^a", "Regex to exclude URLs for downloader")
	flag.StringVar(&downloadExtensionsRegexString, "downloadExtension", ".*", "Regex to match extensions of URL for downloader")
	flag.StringVar(&outputDir, "output", "output", "Output directory location")
	flag.BoolVar(&download, "download", false, "Enable/Disable saving requests")

	flag.Parse()

	crawlIncludePathRegex = regexp.MustCompile(crawlIncludePathRegexString)
	crawlExcludePathRegex = regexp.MustCompile(crawlExcludePathRegexString)
	downloadExtensionsRegex = regexp.MustCompile(downloadExtensionsRegexString)
	downloadIncludePathRegex = regexp.MustCompile(downloadIncludePathRegexString)
	downloadExcludePathRegex = regexp.MustCompile(downloadExcludePathRegexString)

	if len(startURLs) == 0 {
		fmt.Println("Error:\n\tNo URL specified.")
		os.Exit(2)
	}
}

func main() {
	flagParse()
	startCrawler(startURLs)
}
