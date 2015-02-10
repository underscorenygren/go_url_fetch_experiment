/*
package url_fetcher Finds search terms by crawling urls. 

Takes a CSV input file of urls and crawls them in a buffered way. 
Stores resulting crawles and queries them for a search term provided to
the program.
*/
package main

import (
  "fmt"
  "os"
  "flag"
  "encoding/csv"
  "regexp"
  "net/http"
  "io/ioutil"
)

//No limit on number of urls crawled
const NoLimit = -1
const Debug = true

//Structure of a discovered page
type Page struct {
  url string
  body string
  err bool
}

//A simple debug function. In a more robust program, this should be
//written in such a way that debug statments are never executed
func debug(text string) {
  if Debug {
    fmt.Println(text)
  }
}

//Reads a single url into internal representation
func readUrl(url string) *Page {
  var pageError bool
  var body string
  if match, _ := regexp.MatchString("^http", url); match != true {
    url = "http://" + url
  }
  resp, err := http.Get(url)
  debug(fmt.Sprintf("reading url %s", url))
  if err != nil {
    body = fmt.Sprintf("%s", err)
    pageError = true
  } else {
    defer resp.Body.Close()
    bytes, err := ioutil.ReadAll(resp.Body)
    if err != nil {
      pageError = true
      body = fmt.Sprintf("%s", err)
    } else {
      body = fmt.Sprintf("%s", bytes)
      pageError = false
    }
  }
  page := Page{url: url, body: body, err: pageError}
  if page.err {
    debug(page.body)
  }

  return &page
}

//Parses all urls provided into objects
func findUrls(urls chan string, pages chan *Page) {
  debug("crawling urls")
  for url := range urls {
    pages <- readUrl(url)
  }
  close(pages)
}

//Reads file of urls into channels for downloading
func readUrlFile(infile *string, urls chan string, limit int) {
  const urlFieldName = "URL"
  fmt.Printf("Reading urls file: %s\n", *infile)
  nUrls := 0
  file, err := os.Open(*infile)
  defer file.Close()

  if err != nil {
    fmt.Printf("Url file could not be opened: %s\n", err)
    return
  }

  reader := csv.NewReader(file)
  rawCSV, err := reader.ReadAll()
  if err != nil {
    fmt.Printf("Error when loading csv: %s", err)
    return
  }

  //Reading CSV's with this library is funky as hell
  for _, each := range rawCSV {
    if limit != NoLimit && nUrls >= limit {
      debug("breaking because hit limit")
      break
    }
    if err != nil {
      fmt.Printf("Error reading record: %s\n", err)
    } else {
      url := each[1]
      if url != urlFieldName {
        debug(fmt.Sprintf("Found url %s", url))
        urls <- url
        nUrls += 1
      }
    }
  }
  debug("done reading")

  close(urls)
}

//Simple test if a regex term is found in a section of text
func hasTerm(regex string, body string) bool {
  matched, err := regexp.MatchString(regex, body)
  if err != nil {
    fmt.Printf("Error in term match: %s\n", err)
  }
  return matched && err == nil
}

//Finds the search term in all pages
func findTerm(regex string, pages chan *Page, outfile *os.File, done chan bool) {
  nTerms := 0
  for page := range pages {
    if !page.err && hasTerm(regex, page.body) {
      nTerms += 1
      notice := fmt.Sprintf("%s has term\n", page.url)
      outfile.WriteString(notice)
      debug(notice)
    } else {
      debug(fmt.Sprintf("%s doesn't have term", page.url))
    }
  }

  fmt.Printf("Found %d pages with term %s\n", nTerms, regex)
  done <- true
}

//The main function
func main() {
  const NoTerm = "[NO TERM]"
  const MaxConcurrentFetches = 20
  const OutfileName = "results.txt"
  const urlStoreFile = ".urlstore"

  infile := flag.String("infile", "urls.txt", "Location of url source file")
  searchTerm := flag.String("term", NoTerm, "The search term to look for")
  crawlLimit := flag.Int("limit", NoLimit, "The max urls to crawl")
  outfileName := flag.String("outfile", OutfileName, "The name of ther result file")

  flag.Parse()

  if *searchTerm == NoTerm {
    fmt.Println("No search term specified")
    return
  }

  outfile, err := os.Create("results.txt")
  if err != nil {
    fmt.Printf("Could not open %s for writing results\n", outfileName)
    return
  }
  urls := make(chan string)
  pages := make(chan *Page, MaxConcurrentFetches)
  done := make(chan bool)
  //Make regexes case insensitive
  escaped := "(?i)" + regexp.QuoteMeta(*searchTerm)

  fmt.Printf("Searching for term %s in file %s\n", *searchTerm, *infile);
  go readUrlFile(infile, urls, *crawlLimit)
  go findUrls(urls, pages)
  go findTerm(escaped, pages, outfile, done)

  _ = <-done
  fmt.Println("done")

}
