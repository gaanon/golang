package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"

	"github.com/spf13/cobra"
	"golang.org/x/net/html"
)

func traverse(n *html.Node) []string {
	var links []string
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, attr := range n.Attr {
			if attr.Key == "href" {
				links = append(links, attr.Val)
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		links = append(links, traverse(c)...) // ... unpacks the slice as individual items
	}

	return links
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "crawler",
		Short: "Go web crawler",
		Run: func(cmd *cobra.Command, args []string) {
			var links []string
			var uniqueLinks []string

			res, err := http.Get("https://clearroute.io")
			if err != nil {
				log.Fatal(err)
			}
			defer res.Body.Close()

			body, err := io.ReadAll(res.Body)
			if err != nil {
				log.Fatal(err)
			}

			if res.StatusCode > 299 {
				log.Fatalf("Response failed with status code: %d and\nbody: %s\n", res.StatusCode, body)
			}

			doc, err := html.Parse(bytes.NewReader(body))
			if err != nil {
				log.Fatal(err)
			}

			links = traverse(doc)

			var httpsRegex = regexp.MustCompile(`^https://clearroute.io`)
			for _, link := range links {
				if httpsRegex.MatchString(link) {
					uniqueLinks = append(uniqueLinks, link)
					slices.Sort(uniqueLinks)
					uniqueLinks = slices.Compact(uniqueLinks) // Remove duplicates
				}
			}

			for _, link := range uniqueLinks {
				fmt.Println(link)
			}
		},
	}

	rootCmd.Flags().StringP("url", "u", "", "URL to crawl")
	rootCmd.MarkFlagRequired("url")
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Command execution failed: %v", err)
	}
}
