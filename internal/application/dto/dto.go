package dto

type LinkUpdate struct {
	ID          int64
	URL         string
	Description string
	TGChatIDs   []int64
}

type DeleteLink struct {
	Link string
}

type DeleteLinksResponse = TrackedLink

type TrackedLinksResponse struct {
	Links []TrackedLink
}

type TrackedLink struct {
	ID      int64
	URL     string
	Tags    []string
	Filters []string
}

type AddLink struct {
	Link    string
	Tags    []string
	Filters []string
}

type AddLinksResponse = TrackedLink

type RegisterChat struct {
	ChatID int64
}

type DeleteChat struct {
	ChatID int64
}
