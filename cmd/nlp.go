package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
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

func standardize(words []string, input string) []string {
	for _, word := range strings.Fields(input) {
		words = append(words, strings.ToLower(word))
	}
	return words
}

func nlp_index(coly *colly.Collector, channel chan map[string]int, url string) {
	result := make(map[string]int)
	var words []string
	coly.OnHTML("title", func(h *colly.HTMLElement) {
		words = standardize(words, h.Text)
	})
	coly.OnHTML("img", func(h *colly.HTMLElement) {
		altText := h.Attr("alt")
		if altText != "" {
			words = standardize(words, altText)
		}
	})
	coly.OnHTML("meta[name=description]", func(h *colly.HTMLElement) {
		words = standardize(words, h.Attr("content"))
	})
	coly.OnHTML("h1, h2, h3", func(h *colly.HTMLElement) {
		headingEntry := h.Name + " " + h.Text
		if headingEntry != "" {
			words = standardize(words, headingEntry)
		}
	})
	coly.Visit(url)
	coly.Wait()
	for i, word := range words {
		words[i] = removePunctuation(word)
	}
	//execute python program get the result, perform indexing, save words to map and finally return.
	jsonData, _ := json.Marshal(words)
	var python_out bytes.Buffer
	var python_err bytes.Buffer
	var python_result []string
	cmd := exec.Command("python", "cmd/standardize.py")
	cmd.Stdin = bytes.NewReader(jsonData)
	cmd.Stderr = &python_err
	cmd.Stdout = &python_out
	if err := cmd.Run(); err != nil {
		fmt.Printf("Python STDERR:\n%s\n", python_err.String())
		channel <- result
		return
	}
	trimmedOutput := bytes.TrimSpace(python_out.Bytes())
	if err := json.Unmarshal(trimmedOutput, &python_result); err != nil {
		for _, val := range words {
			index(result, val)
		}
		channel <- result
		return
	}
	for _, val := range python_result {
		index(result, val)
	}
	channel <- result
}
