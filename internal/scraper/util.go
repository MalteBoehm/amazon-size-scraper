package scraper

import (
    "regexp"
)

var asinRegex = regexp.MustCompile(`(?i)(?:https?://)?(?:www\.)?amazon\.de/.*/dp/([A-Z0-9]{10})`)

func extractASINFromURL(url string) string {
    if url == "" {
        return ""
    }
    m := asinRegex.FindStringSubmatch(url)
    if len(m) >= 2 {
        return m[1]
    }
    return ""
}

