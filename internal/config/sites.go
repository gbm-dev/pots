package config

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Site represents a remote console site.
type Site struct {
	Name        string
	Phone       string
	Description string
	BaudRate    int
}

// ParseSites reads site definitions from r.
// Each non-blank, non-comment line must be: name|phone|description|baud_rate
func ParseSites(r io.Reader) ([]Site, error) {
	var sites []Site
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			return nil, fmt.Errorf("line %d: expected 4 pipe-delimited fields, got %d", lineNum, len(parts))
		}
		baud, err := strconv.Atoi(strings.TrimSpace(parts[3]))
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid baud rate %q: %w", lineNum, parts[3], err)
		}
		sites = append(sites, Site{
			Name:        strings.TrimSpace(parts[0]),
			Phone:       strings.TrimSpace(parts[1]),
			Description: strings.TrimSpace(parts[2]),
			BaudRate:    baud,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading sites: %w", err)
	}
	return sites, nil
}

// ParseSitesFile reads site definitions from a file path.
func ParseSitesFile(path string) ([]Site, error) {
	f, err := openFile(path)
	if err != nil {
		return nil, fmt.Errorf("opening sites file: %w", err)
	}
	defer f.Close()
	return ParseSites(f)
}
