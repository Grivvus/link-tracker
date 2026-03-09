package dto

type LinkUpdate struct {
	ID          int64
	URL         string
	Description string
	TGChatIDs   []int64
}

type DeleteLink struct {
	ChatID int64
	Link   string
}

type DeleteLinkResponse = TrackedLink

type TrackedLinksResponse = []TrackedLink

type TrackedLink struct {
	ID   int64
	URL  string
	Tags []string
}

type AddLink struct {
	ChatID int64
	Link   string
	Tags   []string
}

type AddLinkResponse = TrackedLink
