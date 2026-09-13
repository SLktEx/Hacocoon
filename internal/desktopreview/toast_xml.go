package desktopreview

import (
	"encoding/xml"
	"strings"
)

// ToastXML uses only literal text and native foreground COM actions. There are
// no URLs, images, protocol activations, script expressions or controller tokens.
func ToastXML(page ToastPage) (string, error) {
	if !hexToken(page.Nonce) || len(page.Inputs) > 5 || len(page.Buttons) > 5 {
		return "", ErrInvalid
	}
	type text struct {
		MaxLines int    `xml:"hint-maxLines,attr,omitempty"`
		Text     string `xml:",chardata"`
	}
	type selection struct {
		ID      string `xml:"id,attr"`
		Content string `xml:"content,attr"`
	}
	type input struct {
		ID      string      `xml:"id,attr"`
		Type    string      `xml:"type,attr"`
		Title   string      `xml:"title,attr"`
		Default string      `xml:"defaultInput,attr"`
		Choices []selection `xml:"selection"`
	}
	type action struct {
		Content    string `xml:"content,attr"`
		Arguments  string `xml:"arguments,attr"`
		Activation string `xml:"activationType,attr"`
	}
	type binding struct {
		Template string `xml:"template,attr"`
		Text     []text `xml:"text"`
	}
	type visual struct {
		Binding binding `xml:"binding"`
	}
	type actions struct {
		Inputs  []input  `xml:"input"`
		Buttons []action `xml:"action"`
	}
	value := struct {
		XMLName  xml.Name `xml:"toast"`
		Duration string   `xml:"duration,attr"`
		Visual   visual   `xml:"visual"`
		Actions  actions  `xml:"actions"`
	}{Duration: "long", Visual: visual{binding{"ToastGeneric", []text{{1, page.Title}, {4, "· " + strings.ReplaceAll(page.Body, "\n", "\n· ")}}}}}
	// A fixed literal prefix on each body line also prevents ms-resource: text
	// on a continuation page from being interpreted as a resource reference.
	for _, field := range page.Inputs {
		if field.ID == "" || len(field.Choices) == 0 || len(field.Choices) > 5 {
			return "", ErrInvalid
		}
		i := input{ID: field.ID, Type: "selection", Title: field.Label, Default: field.Default}
		found := false
		for _, choice := range field.Choices {
			i.Choices = append(i.Choices, selection{choice.ID, choice.Label})
			found = found || choice.ID == field.Default
		}
		if !found {
			return "", ErrInvalid
		}
		value.Actions.Inputs = append(value.Actions.Inputs, i)
	}
	for _, button := range page.Buttons {
		if !strings.HasPrefix(button.Argument, page.Nonce+":") {
			return "", ErrInvalid
		}
		value.Actions.Buttons = append(value.Actions.Buttons, action{button.Label, button.Argument, "foreground"})
	}
	data, err := xml.Marshal(value)
	if err != nil || len(data) > 16<<10 {
		return "", ErrInvalid
	}
	return string(data), nil
}
