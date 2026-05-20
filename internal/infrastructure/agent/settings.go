package agent

import (
	"os"

	"github.com/BurntSushi/toml"
)

type AgentSettings struct {
	Filtering     Filtering     `toml:"filtering"`
	Summarization Summarization `toml:"summarization"`
}

type Filtering struct {
	StopWords       []string `toml:"stop-words"`
	ExcludedAuthors []string `toml:"excluded-authors"`
	MinLength       int      `toml:"min-length"`
}

type Summarization struct {
	Threshold uint `toml:"threshold"`
}

func NewAgentSettings(settingsFile string) (AgentSettings, error) {
	var s AgentSettings
	f, err := os.Open(settingsFile)
	if err != nil {
		return s, err
	}
	defer func() { _ = f.Close() }()
	_, err = toml.NewDecoder(f).Decode(&s)
	return s, err
}
