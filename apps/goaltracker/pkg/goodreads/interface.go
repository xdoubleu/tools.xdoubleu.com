package goodreads

type Client interface {
	GetUserID(profileURL string) (*string, error)
	GetBooks(userID string) ([]Book, error)
}
