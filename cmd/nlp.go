package cmd

import (
	"sync"

	"github.com/gocolly/colly/v2"
)

func nlp_index(coly *colly.Collector, channel chan []string, innerWg *sync.WaitGroup, url string) {
	defer innerWg.Done()
	var result []string
	coly.OnHTML("title", func(h *colly.HTMLElement) {
		result = append(result, h.Text)
	})
	coly.OnHTML("img", func(h *colly.HTMLElement) {
		altText := h.Attr("alt")
		if altText != "" {
			result = append(result, altText)
		}
	})
	coly.OnHTML("meta[name=description]", func(h *colly.HTMLElement) {
		result = append(result, h.Attr("content"))
	})
	coly.OnHTML("h1, h2, h3", func(h *colly.HTMLElement) {
		headingEntry := h.Name + " " + h.Text
		if headingEntry != "" {
			result = append(result, headingEntry)
		}
	})
	coly.Visit(url)
	coly.Wait()
	channel <- result
}
