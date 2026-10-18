package application

import (
	"strings"
)

type Route struct {
	Bot         string   `json:"bot,omitempty"`
	Choices     []string `json:"choices"`
	NeedsChoice bool     `json:"needs_choice"`
}

func (c Catalog) Route(input, explicit string) (Route, error) {
	if explicit != "" {
		b, err := c.Registry.Get(explicit)
		return Route{Bot: b.ID, Choices: []string{b.ID}}, err
	}
	bots, err := c.Registry.List()
	if err != nil {
		return Route{}, err
	}
	mentions := map[string]bool{}
	for _, word := range strings.Fields(strings.ToLower(input)) {
		mentions[strings.Trim(word, ".,:;!?()[]")] = true
	}
	result := Route{Choices: []string{}}
	for _, b := range bots {
		if mentions["@"+b.ID] {
			result.Choices = append(result.Choices, b.ID)
		}
	}
	if len(result.Choices) == 1 {
		result.Bot = result.Choices[0]
		return result, nil
	}
	if len(result.Choices) == 0 {
		for _, b := range bots {
			result.Choices = append(result.Choices, b.ID)
		}
	}
	result.NeedsChoice = true
	return result, nil
}
