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

type PythonResult struct {
	Tokenized []string `json:"result"`
}

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

func standardize(words []string, input string) {
	for _, word := range strings.Fields(input) {
		words = append(words, strings.ToLower(word))
	}
}

func nlp_index(coly *colly.Collector, channel chan map[string]int, url string) {
	result := make(map[string]int)
	var words []string
	coly.OnHTML("title", func(h *colly.HTMLElement) {
		standardize(words, h.Text)
	})
	coly.OnHTML("img", func(h *colly.HTMLElement) {
		altText := h.Attr("alt")
		if altText != "" {
			standardize(words, altText)
		}
	})
	coly.OnHTML("meta[name=description]", func(h *colly.HTMLElement) {
		standardize(words, h.Attr("content"))
	})
	coly.OnHTML("h1, h2, h3", func(h *colly.HTMLElement) {
		headingEntry := h.Name + " " + h.Text
		if headingEntry != "" {
			standardize(words, headingEntry)
		}
	})
	coly.Visit(url)
	coly.Wait()
	for i, word := range words {
		words[i] = removePunctuation(word)
	}
	//execute python program get the result, perform indexing, save words to map and finally return.
	jsonData, _ := json.Marshal(words)
	cmd := exec.Command("python", "cmd/standardize.py")
	cmd.Stdin = bytes.NewReader(jsonData)
	fmt.Print(string(jsonData))
	var python_err bytes.Buffer
	cmd.Stderr = &python_err
	if err := cmd.Run(); err != nil {
		fmt.Printf("Python STDERR:\n%s\n", python_err.String())
		channel <- result
		return
	}
	var python_out bytes.Buffer
	var python_result PythonResult
	cmd.Stdout = &python_out
	fmt.Println(python_out.Bytes())
	fmt.Println(python_result)
	if err := json.Unmarshal(python_out.Bytes(), &python_result); err != nil {
		fmt.Println(err)
		for _, val := range words {
			index(result, val)
		}
		channel <- result
		return
	}
	fmt.Println(python_result.Tokenized)
	for _, val := range python_result.Tokenized {
		index(result, val)
	}
	channel <- result
}
