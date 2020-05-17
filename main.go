package main

import (
	"fmt"
	"golang.org/x/net/html"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

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
	for i, value := range values {
		values[i] = getAbsoluteURL(pageURL, value)
	}
	return values
}

func process(url string) {
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

	if !isHTML {
		return
	}
	body := string(bytes)
	var links = getLinks(url, body)
	fmt.Println(body, "\n\n\n")

	fmt.Println("[+] Links")
	for _, link := range links {
		fmt.Println(" >", link)
	}

	_ = body
	_ = isHTML
}

func main() {
	fmt.Println("--------------------------------------")
	url := "http://localhost/"
	process(url)
}
