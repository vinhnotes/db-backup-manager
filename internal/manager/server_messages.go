package manager

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
)

// Server messages are written in Vietnamese. server_messages.json holds their
// English and Japanese versions as [vi, en, ja]; the web UI uses the same file.
// {0}, {1}… stand for the dynamic parts.
//
//go:embed server_messages.json
var serverMessagesJSON []byte

type serverMessage struct {
	re     *regexp.Regexp
	en, ja string // regexp replacement templates
}

var serverMessages = compileServerMessages(serverMessagesJSON)

var placeholder = regexp.MustCompile(`\{(\d)\}`)

func compileServerMessages(data []byte) []serverMessage {
	var rows [][3]string
	if err := json.Unmarshal(data, &rows); err != nil {
		panic("server_messages.json: " + err.Error())
	}
	out := make([]serverMessage, 0, len(rows))
	for _, r := range rows {
		parts := placeholder.Split(r[0], -1)
		var b strings.Builder
		for i, p := range parts {
			if i > 0 {
				// A placeholder at the very end takes the rest of the line.
				if i == len(parts)-1 && p == "" {
					b.WriteString("(.*)")
				} else {
					b.WriteString("(.*?)")
				}
			}
			b.WriteString(regexp.QuoteMeta(p))
		}
		template := func(s string) string {
			s = strings.ReplaceAll(s, "$", "$$")
			return placeholder.ReplaceAllStringFunc(s, func(m string) string {
				return "${" + string(m[1]-'0'+'1') + "}"
			})
		}
		out = append(out, serverMessage{regexp.MustCompile(b.String()), template(r[1]), template(r[2])})
	}
	return out
}

// translateServer translates known Vietnamese server messages in text (also
// inside longer job errors) into lang. Unknown text is left as is.
func translateServer(lang, text string) string {
	if text == "" || (lang != "en" && lang != "ja") {
		return text
	}
	for _, m := range serverMessages {
		tmpl := m.en
		if lang == "ja" {
			tmpl = m.ja
		}
		text = m.re.ReplaceAllString(text, tmpl)
	}
	return text
}
