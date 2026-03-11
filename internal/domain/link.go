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
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return Link{}, fmt.Errorf("%w: url has invalid format", ErrBadURL)
	}
	var (
		lType     linkType
		ghURLInfo *GitHubURLInfo
		stURLInfo *StackoverflowURLInfo
	)
	if strings.Contains(rawURL, "github") {
		ghinfo, err := fetchGitHubInfo(parsed)
		if err != nil {
			return Link{}, fmt.Errorf(
				"%w: can't fetch username and repository name from url",
				err,
			)
		}
		ghURLInfo = &ghinfo
		lType = githubLink
	} else if strings.Contains(rawURL, "stackoverflow") {
		stinfo, err := fetchStackoverflowInfo(parsed)
		if err != nil {
			return Link{}, fmt.Errorf(
				"%w: can't fetch question id from url",
				err,
			)
		}
		lType = stackoverflowLink
		stURLInfo = &stinfo
	} else {
		return Link{}, fmt.Errorf("%w: service is not supported", ErrBadURL)
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
	parts := strings.Split(url.Path, "/")
	if len(parts) != 2 {
		return GitHubURLInfo{}, ErrBadURL
	}
	return GitHubURLInfo{
		Username: parts[0],
		RepoName: parts[1],
	}, nil
}

func fetchStackoverflowInfo(url *url.URL) (StackoverflowURLInfo, error) {
	parts := strings.Split(url.Path, "/")
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
