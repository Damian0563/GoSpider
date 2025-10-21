package cmd

import (
	"strings"
	"unicode"

	"github.com/gocolly/colly/v2"
)

func removePunctuation(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.IsPunct(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func index(result map[string]int, text string) {
	for _, word := range strings.Fields(text) {
		word = removePunctuation(word)
		result[word]++
	}
}

func nlp_index(coly *colly.Collector, channel chan map[string]int, url string) {
	result := make(map[string]int)
	var words []string
	coly.OnHTML("title", func(h *colly.HTMLElement) {
		words = append(words, strings.ToLower(h.Text))
	})
	coly.OnHTML("img", func(h *colly.HTMLElement) {
		altText := h.Attr("alt")
		if altText != "" {
			words = append(words, strings.ToLower(altText))
		}
	})
	coly.OnHTML("meta[name=description]", func(h *colly.HTMLElement) {
		words = append(words, strings.ToLower(h.Attr("content")))
	})
	coly.OnHTML("h1, h2, h3", func(h *colly.HTMLElement) {
		headingEntry := h.Name + " " + h.Text
		if headingEntry != "" {
			words = append(words, strings.ToLower(headingEntry))
		}
	})
	coly.Visit(url)
	coly.Wait()
	for i, word := range words {
		words[i] = removePunctuation(word)
	}
	//execute python program get the result, perform indexing, save words to map and finally return.

	channel <- result
}
