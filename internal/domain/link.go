package domain

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrBadURL = errors.New("bad URL")

type linkType int

const (
	githubLink linkType = iota + 1
	stackoverflowLink
)

type Link struct {
	ID         int64
	URL        *url.URL
	LastUpdate time.Time
	lType      linkType
	ghURLInfo  *GitHubURLInfo
	stURLInfo  *StackoverflowURLInfo
}

type GitHubURLInfo struct {
	Username string
	RepoName string
}

type StackoverflowURLInfo struct {
	QuestionID int64
}

func NewLink(rawURL string) (Link, error) {
	parsed, lType, err := parseAndValidateURL(rawURL)
	if err != nil {
		return Link{}, fmt.Errorf("%w: url has invalid format", ErrBadURL)
	}
	var (
		ghURLInfo *GitHubURLInfo
		stURLInfo *StackoverflowURLInfo
	)
	switch lType {
	case githubLink:
		ghinfo, err := fetchGitHubInfo(parsed)
		if err != nil {
			return Link{}, fmt.Errorf(
				"%w: can't fetch username and repository name from url",
				err,
			)
		}
		ghURLInfo = &ghinfo
	case stackoverflowLink:
		stinfo, err := fetchStackoverflowInfo(parsed)
		if err != nil {
			return Link{}, fmt.Errorf(
				"%w: can't fetch question id from url",
				err,
			)
		}
		stURLInfo = &stinfo
	default:
		return Link{}, fmt.Errorf("%w: unkown link type", ErrBadURL)
	}
	return Link{
		ID:         0,
		URL:        parsed,
		LastUpdate: time.Now().UTC(),
		lType:      lType,
		ghURLInfo:  ghURLInfo,
		stURLInfo:  stURLInfo,
	}, nil
}

func (l *Link) GitHubURLInfo() (bool, *GitHubURLInfo) {
	return l.lType == githubLink, l.ghURLInfo
}

func (l *Link) StackoverflowURLInfo() (bool, *StackoverflowURLInfo) {
	return l.lType == stackoverflowLink, l.stURLInfo
}

func fetchGitHubInfo(url *url.URL) (GitHubURLInfo, error) {
	path := url.Path
	path, _ = strings.CutSuffix(path, "/")
	path, _ = strings.CutPrefix(path, "/")
	parts := strings.Split(path, "/")
	// /User/reponame
	if len(parts) != 2 {
		return GitHubURLInfo{}, ErrBadURL
	}
	return GitHubURLInfo{
		Username: parts[0],
		RepoName: parts[1],
	}, nil
}

func fetchStackoverflowInfo(url *url.URL) (StackoverflowURLInfo, error) {
	path := url.Path
	path, _ = strings.CutSuffix(path, "/")
	path, _ = strings.CutPrefix(path, "/")
	parts := strings.Split(path, "/")
	// /questions/{id}
	if len(parts) != 2 {
		return StackoverflowURLInfo{}, ErrBadURL
	}
	qid, err := strconv.Atoi(parts[1])
	if err != nil {
		return StackoverflowURLInfo{}, fmt.Errorf(
			"%w: can't convert question_id to int", ErrBadURL,
		)
	}
	return StackoverflowURLInfo{
		QuestionID: int64(qid),
	}, nil
}

func parseAndValidateURL(rawURL string) (*url.URL, linkType, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, 0, err
	}
	// so github.com/User/repo also valid
	if parsed.Scheme == "" {
		rawURL = "https://" + rawURL
		parsed, _ = url.Parse(rawURL)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, 0, fmt.Errorf("%w: scheme must be http or https", ErrBadURL)
	}
	if parsed.Hostname() == "github.com" {
		return parsed, githubLink, nil
	}
	if parsed.Hostname() == "stackoverflow.com" {
		return parsed, stackoverflowLink, nil
	}
	return nil, 0, fmt.Errorf("%w: unknown url, host doesn't supported", ErrBadURL)
}
