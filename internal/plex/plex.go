package plex

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// Client notifies Plex of library changes after a successful transcode.
type Client struct {
	baseURL string
	token   string
	log     *slog.Logger
	http    *http.Client
}

// New creates a Plex client. Returns nil if baseURL or token is empty.
func New(baseURL, token string, log *slog.Logger) *Client {
	if baseURL == "" || token == "" {
		return nil
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		log:     log,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// NotifyFileReplaced triggers a Plex library refresh for the section
// containing replacedPath. Non-fatal: logs and returns on any error.
func (c *Client) NotifyFileReplaced(replacedPath string) {
	if c == nil {
		return
	}

	sections, err := c.listSections()
	if err != nil {
		c.log.Warn("plex: list sections failed", "error", err)
		return
	}

	sectionID := c.findSection(sections, replacedPath)
	if sectionID == "" {
		c.log.Warn("plex: no matching section for file", "path", replacedPath)
		return
	}

	if err := c.refreshSection(sectionID); err != nil {
		c.log.Warn("plex: refresh failed", "section_id", sectionID, "error", err)
		return
	}

	c.log.Info("plex: library refresh triggered", "section_id", sectionID, "path", replacedPath)
}

type plexSection struct {
	Key      string `xml:"key,attr"`
	Title    string `xml:"title,attr"`
	Location []struct {
		Path string `xml:"path,attr"`
	} `xml:"Location"`
}

type plexSections struct {
	Sections []plexSection `xml:"Directory"`
}

func (c *Client) listSections() ([]plexSection, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/library/sections", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Plex-Token", c.token)
	req.Header.Set("Accept", "application/xml")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var container struct {
		Directories []plexSection `xml:"Directory"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return container.Directories, nil
}

func (c *Client) findSection(sections []plexSection, filePath string) string {
	cleanFile := filepath.Clean(filePath)
	for _, s := range sections {
		for _, loc := range s.Location {
			if strings.HasPrefix(cleanFile, filepath.Clean(loc.Path)) {
				return s.Key
			}
		}
	}
	return ""
}

func (c *Client) refreshSection(sectionID string) error {
	url := fmt.Sprintf("%s/library/sections/%s/refresh?X-Plex-Token=%s",
		c.baseURL, sectionID, c.token)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
