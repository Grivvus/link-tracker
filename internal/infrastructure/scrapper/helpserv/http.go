package helpserv

import (
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/sql"
)

type HelpServ struct {
	repo *sql.DBSeeder
}

func NewHelpServ(seeder *sql.DBSeeder) HelpServ {
	return HelpServ{repo: seeder}
}

func (s HelpServ) SeedDBWithTestData(w http.ResponseWriter, r *http.Request) {
	start, err := s.repo.SeedLinkAndUsers(r.Context(), 100_000, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	err = s.repo.SeedSubscriptions(r.Context(), start, 100_000, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s HelpServ) TearDownDB(w http.ResponseWriter, r *http.Request) {
	err := s.repo.ClearDB(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
