/*
* GO web crawler made by following this outdated guide: https://jdanger.com/build-a-web-crawler-in-go.html and using the go html 5 parser
	Also multi-threaded, also shows no mercy to RAM or CPU usage, run when server is down
*/
package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/net/html"
)

type nameForWebPage struct {
	WebPage webPage `json:"webpage"`
}
type webPage struct {
	URI      string   `json:"uri"`
	Keywords []string `json:"keywords"`
}

var f, err = os.OpenFile("output.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
var mapOfURIsToKeywords = make(map[string]bool)
var mutex = &sync.Mutex{}
var wg sync.WaitGroup

func main() {
	//Call this to Parse the commands that are passed into the
	flag.Parse()

	args := flag.Args()
	defer f.Close()

	//len() functions a lot like Python's len()
	if len(args) < 1 {
		fmt.Println("Specify a start page as an argument")
		os.Exit(1)
	}
	queue := make(chan string)
	mapOfURIsToKeywords[args[0]] = false
	go func() { queue <- args[0] }()
	for uri := range queue {
		if val, ok := mapOfURIsToKeywords[uri]; !ok || val == false {
			mapOfURIsToKeywords[uri] = true
			go queueUp(uri, queue)

		} else {
			go func() {
				//Wait here until something is added into the queue
				<-queue
			}()
			//Skip this one since it was invalid (already visited)
			continue
		}
	}
	//Kill the program with no errors if the queue empties (if every possible link has been collected)
	os.Exit(1)
}

func collectLinks(node *html.Node, links *[]string) {

	if node.Type == html.ElementNode && node.Data == "a" {

		//range keyword iterates over many different types, this particular case is an iteration over an array where _ corresponds to the index and a corresponds to the element
		for _, a := range node.Attr {

			if a.Key == "href" {
				if strings.Contains(a.Val, "http") {

					*links = append(*links, a.Val)
					break
				}

			}
		}
	}

	for childNode := node.FirstChild; childNode != nil; childNode = childNode.NextSibling {
		collectLinks(childNode, links)
	}

}

//Retrieves the text of a web page and sanitizes it to be valid JSON (hopefully)
func getText(node *html.Node, text *[]string) {
	if node.Type == html.TextNode && node.Data != "script" {
		stringToAdd := strings.TrimSpace(node.Data)
		if stringToAdd != "" && utf8.ValidString(stringToAdd) {
			s := ReplaceSpace(stringToAdd)
			*text = append(*text, strings.TrimSpace(s))
		}
	}

	for childNode := node.FirstChild; childNode != nil; childNode = childNode.NextSibling {
		if childNode.Data != "script" && childNode.Data != "style" {
			getText(childNode, text)
		}
	}
}

func queueUp(uri string, queue chan string) {
	//fmt.Println("Entering queue up: " + uri)
	links := make([]string, 1)
	text := make([]string, 1)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	//GO has the ability to return two values, resp corresponds to the response of the get request, error corresponds to the potential error that could be returned from this function
	client := http.Client{Transport: transport}

	resp, err := client.Get(uri)

	//Standard error checking in GO, if the error that was returned is non-null (i.e it has been populated by something) then do something
	if err != nil {
		fmt.Println(err)
		return
	}
	//Defer keyword means that this line of code will execute at the end of this method's execution
	defer resp.Body.Close()
	defer client.CloseIdleConnections()
	//Use _ when you don't care about the potential for an error being returned by a given function. resp.Body can be thought of a Java bytestream, and the ReadAll function collects the results into a byte[]
	body, _ := ioutil.ReadAll(resp.Body)

	r := bytes.NewReader(body)

	htmlParser, err := html.ParseWithOptions(r, html.ParseOptionEnableScripting(false))

	if err != nil {
		log.Fatal(err)
	}

	collectLinks(htmlParser, &links)
	getText(htmlParser, &text)

	go func() {
		thisPage := nameForWebPage{
			WebPage: webPage{URI: uri, Keywords: text},
		}
		pageAsJson, err := json.Marshal(thisPage)
		if err != nil {
			log.Fatal("Cannot marshal JSON object: " + string(pageAsJson))
		}

		if _, err = f.WriteString(string(pageAsJson)); err != nil {
			log.Fatal("Unable to write to file")
		}
	}()
	for _, link := range links {
		queue <- link

	}
}

//https://stackoverflow.com/questions/27931884/convert-normal-space-whitespace-to-non-breaking-space-in-golang
//Needed for removing non-breaking whitespace unicode character, as well as quote marks from strings
func ReplaceSpace(s string) string {
	var result []rune
	const badSpace = '\u0020'
	const otherBadSpace = '\u00A0'
	const quotes = '"'
	for _, r := range s {
		if r == badSpace || r == otherBadSpace || r == quotes {
			result = append(result, ' ')
			continue
		}
		result = append(result, r)
	}
	return string(result)
}
