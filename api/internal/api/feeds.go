package api

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gongahkia/tanabata/api/internal/models"
)

const atomMediaType = "application/atom+xml; charset=utf-8"

type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Author  atomAuthor  `xml:"author"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
}

type atomEntry struct {
	ID      string      `xml:"id"`
	Title   string      `xml:"title"`
	Updated string      `xml:"updated"`
	Summary string      `xml:"summary"`
	Content atomContent `xml:"content"`
}

type atomContent struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

func (s *Server) disputesAtom(c *gin.Context) {
	disputes, err := s.store.Disputes(c.Request.Context(), parseLimit(c.Query("limit"), 20))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "disputes_failed", "failed to load disputes", nil, err)
		return
	}
	feed, updated, err := disputeAtomFeed(disputes, feedSelfURL(c), feedAlternateURL(c))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "disputes_feed_failed", "failed to serialize disputes feed", nil, err)
		return
	}
	payload, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "disputes_feed_failed", "failed to encode disputes feed", nil, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.Header("Last-Modified", updated.Format(http.TimeFormat))
	c.Data(http.StatusOK, atomMediaType, append([]byte(xml.Header), payload...))
}

func disputeAtomFeed(disputes []models.Dispute, selfURL, alternateURL string) (atomFeed, time.Time, error) {
	updated := time.Unix(0, 0).UTC()
	entries := make([]atomEntry, 0, len(disputes))
	for _, dispute := range disputes {
		entryUpdated := claimUpdatedAt(dispute.Claim)
		if entryUpdated.After(updated) {
			updated = entryUpdated
		}
		content, err := json.Marshal(dispute.Claim)
		if err != nil {
			return atomFeed{}, time.Time{}, err
		}
		entries = append(entries, atomEntry{
			ID:      atomClaimID(dispute.Claim.ClaimID, entryUpdated),
			Title:   dispute.HumanDescription,
			Updated: entryUpdated.Format(time.RFC3339),
			Summary: dispute.Claim.Kind + " " + dispute.Claim.Status,
			Content: atomContent{
				Type: "application/json",
				Body: string(content),
			},
		})
	}
	feed := atomFeed{
		Title:   "Tanabata disputes",
		ID:      "tag:tanabata.dev," + updated.Format("2006") + ":disputes",
		Updated: updated.Format(time.RFC3339),
		Author:  atomAuthor{Name: "Tanabata"},
		Links: []atomLink{
			{Href: selfURL, Rel: "self", Type: "application/atom+xml"},
			{Href: alternateURL, Rel: "alternate", Type: "application/json"},
		},
		Entries: entries,
	}
	return feed, updated, nil
}

func atomClaimID(claimID string, updated time.Time) string {
	return "tag:tanabata.dev," + updated.Format("2006") + ":claim/" + claimID
}

func claimUpdatedAt(claim models.Claim) time.Time {
	for _, value := range []string{claim.UpdatedAt, claim.LastVerifiedAt, claim.AssertedAt} {
		if parsed, ok := parseFeedTimestamp(value); ok {
			return parsed
		}
	}
	return time.Unix(0, 0).UTC()
}

func parseFeedTimestamp(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), true
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed.UTC(), true
	}
	return time.Time{}, false
}

func feedSelfURL(c *gin.Context) string {
	return feedBaseURL(c) + c.Request.URL.Path
}

func feedAlternateURL(c *gin.Context) string {
	return feedBaseURL(c) + "/v1/disputes"
}

func feedBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(c.Request.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = forwarded
	}
	return scheme + "://" + c.Request.Host
}
