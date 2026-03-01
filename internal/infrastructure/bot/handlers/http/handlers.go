package bothttp

import "net/http"

type BotServer struct{}

func (b *BotServer) PostUpdates(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
