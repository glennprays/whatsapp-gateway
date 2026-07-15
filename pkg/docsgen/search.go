package docsgen

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SearchRecord is one searchable section of a doc: the text between one H1/H2
// heading and the next. Chunking on H1/H2 keeps HeadingID aligned with the
// anchor ids app.js assigns (it only slugs h1/h2), so a result navigates to
// #docs:<File>:<HeadingID> and lands on the right section.
type SearchRecord struct {
	File      string `json:"file"`      // doc key, e.g. "security/authentication-and-security"
	DocTitle  string `json:"docTitle"`  // the doc's first H1 (fallback: humanized file name)
	Heading   string `json:"heading"`   // this section's heading text ("" for pre-heading intro)
	HeadingID string `json:"headingId"` // slug of Heading, matching app.js generateSlug ("" for intro)
	Text      string `json:"text"`      // plain-text body of the section
}

var (
	reLink    = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`) // [text](url) / ![alt](src) -> text/alt
	reListNum = regexp.MustCompile(`^\s*\d+\.\s+`)            // "1. "
	reSpaces  = regexp.MustCompile(`[ \t]+`)
	reHeading = regexp.MustCompile(`^(#{1,2})\s+(.*)$`) // H1/H2 only (matches app.js anchors)
	reSlugBad = regexp.MustCompile(`[^\w\s-]`)          // matches app.js: strip non-word/space/hyphen
	reSlugWs  = regexp.MustCompile(`\s+`)
	reSlugDup = regexp.MustCompile(`-+`)
)

// Slugify mirrors app.js generateSlug so search results anchor to the same ids
// that addHeadingAnchors assigns in the browser.
func Slugify(text string) string {
	s := strings.ToLower(text)
	s = reSlugBad.ReplaceAllString(s, "")
	s = reSlugWs.ReplaceAllString(s, "-")
	s = reSlugDup.ReplaceAllString(s, "-")
	return strings.Trim(s, " ") // JS trim only strips whitespace, but spaces are gone by now
}

// BuildSearchIndex walks docsDir and returns one SearchRecord per H1/H2 section
// of every markdown doc. ```mermaid fenced blocks are dropped (diagram source is
// not prose); other fenced code is kept as searchable text (env vars, curl, etc).
func BuildSearchIndex(docsDir string) ([]SearchRecord, error) {
	keys, err := ListMarkdown(docsDir)
	if err != nil {
		return nil, err
	}
	var records []SearchRecord
	for _, key := range keys {
		f, err := os.Open(filepath.Join(docsDir, filepath.FromSlash(key)+".md"))
		if err != nil {
			return nil, err
		}
		recs := parseDoc(key, f)
		f.Close()
		records = append(records, recs...)
	}
	return records, nil
}

// parseDoc splits one markdown doc into section records.
func parseDoc(key string, r io.Reader) []SearchRecord {
	docTitle := humanize(filepath.Base(key))
	var records []SearchRecord
	cur := SearchRecord{File: key} // pre-heading intro section
	var body strings.Builder

	inFence := false
	fenceIsMermaid := false

	flush := func() {
		cur.Text = cleanText(body.String())
		body.Reset()
		if cur.Text != "" || cur.Heading != "" {
			records = append(records, cur)
		}
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()

		if fence := strings.TrimSpace(line); strings.HasPrefix(fence, "```") {
			if !inFence {
				inFence = true
				fenceIsMermaid = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(fence, "```")), "mermaid")
			} else {
				inFence = false
				fenceIsMermaid = false
			}
			continue // drop fence markers themselves
		}
		if inFence {
			if !fenceIsMermaid {
				body.WriteString(line)
				body.WriteByte('\n')
			}
			continue
		}

		if m := reHeading.FindStringSubmatch(line); m != nil {
			heading := strings.TrimSpace(stripInline(m[2]))
			// The first H1 becomes the doc title for every record.
			if len(m[1]) == 1 && docTitleIsDefault(docTitle, key) {
				docTitle = heading
			}
			flush()
			cur = SearchRecord{File: key, Heading: heading, HeadingID: Slugify(heading)}
			continue
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	flush()

	for i := range records {
		records[i].DocTitle = docTitle
	}
	return records
}

func docTitleIsDefault(cur, key string) bool {
	return cur == humanize(filepath.Base(key))
}

// stripInline removes inline markdown markers from a heading, leaving readable
// text. Underscores are preserved (env-var names like HMAC_MASTER_KEY).
func stripInline(s string) string {
	s = reLink.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "*", "")
	return strings.TrimSpace(s)
}

// cleanText flattens markdown body lines into searchable plain text.
func cleanText(s string) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		t = reLink.ReplaceAllString(t, "$1")
		t = strings.ReplaceAll(t, "`", "")
		t = strings.ReplaceAll(t, "**", "")
		t = strings.Trim(t, ">|")             // blockquote / table pipes
		t = strings.TrimLeft(t, "-*+ ")       // bullet markers
		t = reListNum.ReplaceAllString(t, "") // "1. "
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	joined := strings.Join(out, " ")
	return strings.TrimSpace(reSpaces.ReplaceAllString(joined, " "))
}

// humanize turns a file base like "quick-start" into "Quick Start".
func humanize(base string) string {
	parts := strings.FieldsFunc(base, func(r rune) bool { return r == '-' || r == '_' })
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
