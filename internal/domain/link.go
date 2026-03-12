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
	parts := strings.Split(url.Path[1:], "/")
	if len(parts) != 2 {
		return GitHubURLInfo{}, ErrBadURL
	}
	return GitHubURLInfo{
		Username: parts[0],
		RepoName: parts[1],
	}, nil
}

func fetchStackoverflowInfo(url *url.URL) (StackoverflowURLInfo, error) {
	parts := strings.Split(url.Path[1:], "/")
	if len(parts) != 1 {
		return StackoverflowURLInfo{}, ErrBadURL
	}
	qid, err := strconv.Atoi(parts[0])
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
